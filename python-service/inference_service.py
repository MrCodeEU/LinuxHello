#!/usr/bin/env python3
"""
LinuxHello Inference Service
Provides face detection and recognition via gRPC
Supports ROCm, DirectML, and CPU execution
"""

import os
import time
import logging
from concurrent import futures
from typing import List, Tuple, Optional

import numpy as np
import cv2
import grpc
import onnxruntime as ort

# Import generated protobuf code
import inference_pb2
import inference_pb2_grpc

# Configure logging
logging.basicConfig(
    level=logging.INFO,
    format='%(asctime)s - %(name)s - %(levelname)s - %(message)s'
)
logger = logging.getLogger(__name__)


class FaceDetector:
    """SCRFD face detector"""
    
    def __init__(self, model_path: str, conf_threshold: float = 0.5, nms_threshold: float = 0.4):
        self.conf_threshold = conf_threshold
        self.nms_threshold = nms_threshold
        self.input_size = (640, 640)
        self.debug_mode = os.environ.get('LINUXHELLO_DEBUG', '').lower() == 'true'
        
        # Create ONNX Runtime session with available providers
        providers = self._get_available_providers()
        logger.info(f"Creating face detector with providers: {providers}")
        
        self.session = ort.InferenceSession(model_path, providers=providers)
        self.input_name = self.session.get_inputs()[0].name
        
        # Dynamically classify outputs based on shape to be robust against reordering
        self.score_names = []
        self.bbox_names = []
        self.kps_names = []
        
        # Collect outputs and their sizes (number of anchors)
        scores_with_size = []
        bboxes_with_size = []
        kps_with_size = []
        
        for out in self.session.get_outputs():
            shape = out.shape
            # Handle dynamic batch size (usually 'None' or -1 in index 0)
            # We look at the last dimension to determine type
            
            # Shape patterns for SCRFD:
            # Score: [N, 1]
            # Bbox:  [N, 4]
            # Kps:   [N, 10]
            
            cols = shape[-1]
            # Approximate N based on expected resolution 640x640
            # N is one of 6400, 1600, 400, 100, 25
            
            # Since shape[0] usually contains variable dimension (batch size) in ONNX,
            # we can't trust it to be an integer. It might be None or "BatchSize".
            # We rely on columns.
            
            if cols == 1:
                # Score
                scores_with_size.append(out)
            elif cols == 4:
                # Bbox
                bboxes_with_size.append(out)
            elif cols == 10:
                # Kps
                kps_with_size.append(out)
        
        # Helper to sort by anchor count descending.
        # Since we don't know N from shape (it's often flexible), we assume 
        # that within the ONNX graph, the outputs are ordered by scale or 
        # we can't easily distinguish them solely by metadata if N is dynamic.
        # BUT, usually 'BatchSize' * 'H*W' is the shape.
        
        # To be safe, we will rely on names or standard ONNX export ordering if shape is strictly dynamic.
        # However, typically `session.run` returns actual numpy arrays where we CAN check size.
        # So we should do this sorting at runtime (in detect), OR trust that existing order within groups is correct.
        # Let's trust the grouping first, then at runtime map them strictly by size.
        
        self.output_names = [out.name for out in self.session.get_outputs()]
        
        logger.info(f"Face detector loaded. Input: {self.input_name}, Outputs: {len(self.output_names)}")
        logger.debug(f"Output names: {self.output_names}")

    
    def _get_available_providers(self) -> List[str]:
        """Get available execution providers in priority order"""
        available = ort.get_available_providers()
        logger.info(f"Available ONNX Runtime providers: {available}")
        
        # Priority order: ROCm > DirectML > CUDA > CPU
        preferred = ['ROCMExecutionProvider', 'DmlExecutionProvider', 
                    'CUDAExecutionProvider', 'CPUExecutionProvider']
        
        providers = [p for p in preferred if p in available]
        if not providers:
            providers = ['CPUExecutionProvider']
        
        return providers
    
    def preprocess(self, image: np.ndarray) -> Tuple[np.ndarray, dict]:
        """
        Preprocess image for SCRFD model with optimized scaling.
        
        Strategy: Resize to maintain aspect ratio first, then pad to square.
        This keeps faces at maximum resolution in the model input.
        
        Returns:
            Tuple of (preprocessed_image, transform_info) where transform_info
            contains the padding and scaling information needed to transform
            coordinates back to the original image space.
        """
        h, w = image.shape[:2]
        target_size = self.input_size[0]  # 640
        
        # Calculate scale to fit the longer dimension to target_size
        # while maintaining aspect ratio
        scale = target_size / max(h, w)
        new_w = int(w * scale)
        new_h = int(h * scale)
        
        # Resize image maintaining aspect ratio
        resized = cv2.resize(image, (new_w, new_h), interpolation=cv2.INTER_LINEAR)
        
        # Create square canvas and center the resized image
        canvas = np.zeros((target_size, target_size, 3), dtype=np.uint8)
        y_offset = (target_size - new_h) // 2
        x_offset = (target_size - new_w) // 2
        canvas[y_offset:y_offset+new_h, x_offset:x_offset+new_w] = resized
        
        # Store transform info for coordinate mapping back
        transform_info = {
            'original_width': w,
            'original_height': h,
            'scale': scale,  # Scale factor from original to resized
            'x_offset': x_offset,  # Padding on left
            'y_offset': y_offset,  # Padding on top
            'resized_width': new_w,
            'resized_height': new_h
        }
        
        # Convert to RGB and normalize to [-1, 1] (InsightFace SCRFD standard)
        img = cv2.cvtColor(canvas, cv2.COLOR_BGR2RGB)
        img = (img.astype(np.float32) - 127.5) / 128.0
        
        # Transpose to CHW format
        img = img.transpose(2, 0, 1)
        
        # Add batch dimension
        img = np.expand_dims(img, axis=0)
        
        # Log preprocessing details at debug level
        logger.debug(f"Preprocessing: original={w}x{h} → resized={new_w}x{new_h} → 640x640, "
                   f"scale={scale:.4f}, padding=({x_offset},{y_offset})")
        
        return img, transform_info
    
    def _determine_model_topology(self, scores_out):
        """Determine model architecture (strides and anchors per position)"""
        num_scales = len(scores_out)
        if num_scales == 3:
            strides = [8, 16, 32]
        elif num_scales == 5:
            strides = [8, 16, 32, 64, 128]
        else:
            strides = [2**i * 8 for i in range(num_scales)]
        
        # Detect number of anchors per position from the largest scale
        largest_count = scores_out[0][0]
        feat_size_largest = 640 // strides[0]
        num_anchors = max(1, largest_count // (feat_size_largest * feat_size_largest))
        
        logger.debug(f"Model topology: {num_scales} scales, strides={strides}, num_anchors={num_anchors}")
        return strides, num_anchors
    
    def _decode_bbox(self, bbox_row, cx, cy, stride):
        """Decode bounding box from anchor-relative coordinates"""
        dx1, dy1, dx2, dy2 = bbox_row
        x1 = cx - dx1 * stride
        y1 = cy - dy1 * stride
        x2 = cx + dx2 * stride
        y2 = cy + dy2 * stride
        return x1, y1, x2, y2
    
    def _transform_coords_to_original(self, x, y, transform_info, original_w, original_h):
        """Transform coordinates from 640x640 model space to original image space"""
        # Remove padding offsets
        x_resized = x - transform_info['x_offset']
        y_resized = y - transform_info['y_offset']
        
        # Clamp to resized bounds
        x_resized = max(0, min(x_resized, transform_info['resized_width']))
        y_resized = max(0, min(y_resized, transform_info['resized_height']))
        
        # Scale to original
        x_orig = x_resized / transform_info['scale']
        y_orig = y_resized / transform_info['scale']
        
        # Clamp to original bounds
        x_orig = max(0, min(x_orig, original_w))
        y_orig = max(0, min(y_orig, original_h))
        
        return x_orig, y_orig
    
    def _decode_landmarks(self, kps_row, cx, cy, stride, transform_info, original_w, original_h):
        """Decode 5-point facial landmarks"""
        landmarks = []
        for j in range(5):
            kpx = kps_row[j * 2]
            kpy = kps_row[j * 2 + 1]
            
            lx_640 = cx + kpx * stride
            ly_640 = cy + kpy * stride
            
            lx, ly = self._transform_coords_to_original(
                lx_640, ly_640, transform_info, original_w, original_h
            )
            landmarks.append([lx, ly])
        
        return landmarks
    
    def _process_scale_detections(self, scale_idx, scores_out, bboxes_out, kps_out, 
                                   strides, num_anchors, transform_info, 
                                   original_w, original_h, conf_threshold):
        """Process detections for a single scale"""
        curr_size, score_tensor = scores_out[scale_idx]
        _, bboxes = bboxes_out[scale_idx]
        _, keypoints = kps_out[scale_idx]
        
        stride = strides[scale_idx]
        feat_size = 640 // stride
        
        logger.debug(f"Processing scale {scale_idx}: anchors={curr_size}, "
                   f"feat_size={feat_size}, stride={stride}, num_anchors={num_anchors}")

        scores = score_tensor.reshape(-1)
        keep_indices = np.nonzero(scores >= conf_threshold)[0]
        
        detections = []
        for i in keep_indices:
            score = float(scores[i])
            
            # Calculate grid position
            pos_idx = i // num_anchors
            h_idx = pos_idx // feat_size
            w_idx = pos_idx % feat_size
            
            # Center point in 640x640 space
            cx = (w_idx + 0.5) * stride
            cy = (h_idx + 0.5) * stride
            
            # Decode bbox
            x1_640, y1_640, x2_640, y2_640 = self._decode_bbox(
                bboxes[i], cx, cy, stride
            )
            
            # Transform to original coordinates
            x1, y1 = self._transform_coords_to_original(
                x1_640, y1_640, transform_info, original_w, original_h
            )
            x2, y2 = self._transform_coords_to_original(
                x2_640, y2_640, transform_info, original_w, original_h
            )
            
            # Decode landmarks
            landmarks = self._decode_landmarks(
                keypoints[i], cx, cy, stride, 
                transform_info, original_w, original_h
            )
            
            detections.append({
                'bbox': [x1, y1, x2, y2],
                'confidence': score,
                'landmarks': landmarks
            })
        
        return detections
    
    def _classify_and_sort_outputs(self, outputs_list):
        """Classify model outputs into scores, bboxes, and keypoints"""
        scores_out = []
        bboxes_out = []
        kps_out = []
        
        for arr in outputs_list:
            # Flatten batch dim if present
            if arr.ndim == 3 and arr.shape[0] == 1:
                arr = arr[0]
            
            # Ensure 2D
            if arr.ndim < 2 and len(arr.shape) == 1:
                arr = arr.reshape(-1, 1)

            cols = arr.shape[-1]
            rows = arr.shape[0]
            
            if cols == 1:
                scores_out.append((rows, arr))
            elif cols == 4:
                bboxes_out.append((rows, arr))
            elif cols == 10:
                kps_out.append((rows, arr))
        
        # Sort by number of anchors descending
        scores_out.sort(key=lambda x: x[0], reverse=True)
        bboxes_out.sort(key=lambda x: x[0], reverse=True)
        kps_out.sort(key=lambda x: x[0], reverse=True)
        
        return scores_out, bboxes_out, kps_out

    def detect(self, image: np.ndarray, conf_threshold: Optional[float] = None,
               nms_threshold: Optional[float] = None) -> List[dict]:
        """
        Detect faces in image
        
        Returns:
            List of detections with keys: bbox, confidence, landmarks
        """
        if conf_threshold is None:
            conf_threshold = self.conf_threshold
        if nms_threshold is None:
            nms_threshold = self.nms_threshold
        
        original_h, original_w = image.shape[:2]
        logger.debug(f"Detect called with image size: {original_w}x{original_h}")
        
        # Preprocess
        input_data, transform_info = self.preprocess(image)
        logger.debug(f"Transform info: {transform_info}")
        
        # Run inference
        outputs_list = self.session.run(self.output_names, {self.input_name: input_data})
        
        # Classify and sort outputs
        scores_out, bboxes_out, kps_out = self._classify_and_sort_outputs(outputs_list)
        
        # Verify alignment
        if not (len(scores_out) == len(bboxes_out) == len(kps_out)):
            logger.error(f"Output count mismatch: scores={len(scores_out)}, "
                       f"bboxes={len(bboxes_out)}, kps={len(kps_out)}")
            return []

        # Determine model topology
        strides, num_anchors = self._determine_model_topology(scores_out)
        
        # Process each scale
        detections = []
        try:
            for scale_idx in range(len(scores_out)):
                scale_dets = self._process_scale_detections(
                    scale_idx, scores_out, bboxes_out, kps_out,
                    strides, num_anchors, transform_info,
                    original_w, original_h, conf_threshold
                )
                detections.extend(scale_dets)
        except Exception as e:
            import traceback
            traceback.print_exc()
            logger.error(f"Error parsing detections: {e}")
            raise e
        
        # Apply NMS
        detections = self.nms(detections, nms_threshold)
        
        logger.debug(f"Found {len(detections)} face(s) after NMS")
        if len(detections) > 0:
            logger.info(f"First detection: bbox={detections[0]['bbox']}, "
                      f"conf={detections[0]['confidence']:.3f}")
        
        return detections
    
    @staticmethod
    def nms(detections: List[dict], threshold: float) -> List[dict]:
        """Non-maximum suppression"""
        if len(detections) == 0:
            return []
        
        # Sort by confidence
        detections = sorted(detections, key=lambda x: x['confidence'], reverse=True)
        
        keep = []
        while detections:
            current = detections.pop(0)
            keep.append(current)
            
            # Filter overlapping boxes
            detections = [
                d for d in detections
                if FaceDetector.iou(current['bbox'], d['bbox']) < threshold
            ]
        
        return keep
    
    @staticmethod
    def iou(box1: List[float], box2: List[float]) -> float:
        """Calculate IoU between two boxes"""
        x1_1, y1_1, x2_1, y2_1 = box1
        x1_2, y1_2, x2_2, y2_2 = box2
        
        # Intersection
        x1 = max(x1_1, x1_2)
        y1 = max(y1_1, y1_2)
        x2 = min(x2_1, x2_2)
        y2 = min(y2_1, y2_2)
        
        if x2 <= x1 or y2 <= y1:
            return 0.0
        
        intersection = (x2 - x1) * (y2 - y1)
        area1 = (x2_1 - x1_1) * (y2_1 - y1_1)
        area2 = (x2_2 - x1_2) * (y2_2 - y1_2)
        union = area1 + area2 - intersection
        
        return intersection / union if union > 0 else 0.0


class FaceRecognizer:
    """ArcFace face recognizer"""
    
    def __init__(self, model_path: str):
        self.input_size = (112, 112)
        
        # Create ONNX Runtime session
        providers = self._get_available_providers()
        logger.info(f"Creating face recognizer with providers: {providers}")
        
        self.session = ort.InferenceSession(model_path, providers=providers)
        self.input_name = self.session.get_inputs()[0].name
        self.output_name = self.session.get_outputs()[0].name
        
        logger.info("Face recognizer loaded")
    
    def _get_available_providers(self) -> List[str]:
        """Get available execution providers"""
        available = ort.get_available_providers()
        preferred = ['ROCMExecutionProvider', 'DmlExecutionProvider',
                    'CUDAExecutionProvider', 'CPUExecutionProvider']
        providers = [p for p in preferred if p in available]
        if not providers:
            providers = ['CPUExecutionProvider']
        return providers
    
    def align_face(self, image: np.ndarray, landmarks: List[List[float]]) -> np.ndarray:
        """Align face using 5-point landmarks"""
        # Target landmarks for 112x112 image
        target_landmarks = np.array([
            [38.2946, 51.6963],
            [73.5318, 51.5014],
            [56.0252, 71.7366],
            [41.5493, 92.3655],
            [70.7299, 92.2041]
        ], dtype=np.float32)
        
        src_landmarks = np.array(landmarks, dtype=np.float32)
        
        # Compute similarity transform
        tform = cv2.estimateAffinePartial2D(src_landmarks, target_landmarks)[0]
        
        # Warp image
        aligned = cv2.warpAffine(image, tform, self.input_size, 
                                borderValue=0.0)
        
        return aligned
    
    def extract_embedding(self, image: np.ndarray, landmarks: List[List[float]]) -> np.ndarray:
        """Extract face embedding"""
        # Align face
        aligned = self.align_face(image, landmarks)
        
        # Preprocess
        aligned = cv2.cvtColor(aligned, cv2.COLOR_BGR2RGB)
        aligned = aligned.astype(np.float32) / 255.0
        aligned = aligned.transpose(2, 0, 1)
        aligned = np.expand_dims(aligned, axis=0)
        
        # Run inference
        embedding = self.session.run([self.output_name], {self.input_name: aligned})[0]
        
        # Normalize
        embedding = embedding.flatten()
        norm = np.linalg.norm(embedding)
        if norm > 0:
            embedding = embedding / norm
        
        return embedding


class InferenceServicer(inference_pb2_grpc.FaceInferenceServicer):
    """gRPC servicer implementation"""
    
    def __init__(self, detector_path: str, recognizer_path: str):
        self.detector = FaceDetector(detector_path)
        self.recognizer = FaceRecognizer(recognizer_path)
        self.version = "1.0.0"
        
        # Get device info
        providers = ort.get_available_providers()
        if 'ROCMExecutionProvider' in providers:
            self.device = "rocm"
        elif 'DmlExecutionProvider' in providers:
            self.device = "directml"
        elif 'CUDAExecutionProvider' in providers:
            self.device = "cuda"
        else:
            self.device = "cpu"
        
        logger.info(f"Inference service initialized on device: {self.device}")
    
    def _decode_image(self, image_msg: inference_pb2.Image) -> np.ndarray:
        """Decode image from protobuf message"""
        if image_msg.format in ["jpeg", "png"]:
            # Decode from compressed format
            nparr = np.frombuffer(image_msg.data, np.uint8)
            img = cv2.imdecode(nparr, cv2.IMREAD_COLOR)
        else:
            # Raw pixel data
            img = np.frombuffer(image_msg.data, dtype=np.uint8)
            img = img.reshape((image_msg.height, image_msg.width, image_msg.channels))
            if image_msg.channels == 3:
                img = cv2.cvtColor(img, cv2.COLOR_RGB2BGR)
        
        return img
    
    def DetectFaces(self, request, context):
        """Detect faces in image"""
        try:
            start_time = time.time()
            
            # Decode image
            image = self._decode_image(request.image)
            
            # Detect faces
            conf_threshold = request.confidence_threshold if request.confidence_threshold > 0 else None
            nms_threshold = request.nms_threshold if request.nms_threshold > 0 else None
            
            detections = self.detector.detect(image, conf_threshold, nms_threshold)
            
            # Convert to protobuf
            pb_detections = []
            for det in detections:
                x1, y1, x2, y2 = det['bbox']
                landmarks = [inference_pb2.Landmark(x=lm[0], y=lm[1]) for lm in det['landmarks']]
                
                pb_det = inference_pb2.Detection(
                    x1=x1, y1=y1, x2=x2, y2=y2,
                    confidence=det['confidence'],
                    landmarks=landmarks
                )
                pb_detections.append(pb_det)
            
            inference_time = int((time.time() - start_time) * 1000)
            
            return inference_pb2.DetectResponse(
                detections=pb_detections,
                inference_time_ms=inference_time
            )
            
        except Exception as e:
            logger.error(f"Error in DetectFaces: {e}", exc_info=True)
            context.set_code(grpc.StatusCode.INTERNAL)
            context.set_details(str(e))
            return inference_pb2.DetectResponse()
    
    def ExtractEmbedding(self, request, context):
        """Extract face embedding"""
        try:
            start_time = time.time()
            
            # Decode image
            image = self._decode_image(request.image)
            
            # Extract landmarks from face detection
            landmarks = [[lm.x, lm.y] for lm in request.face.landmarks]
            
            # Extract embedding
            embedding = self.recognizer.extract_embedding(image, landmarks)
            
            inference_time = int((time.time() - start_time) * 1000)
            
            return inference_pb2.EmbeddingResponse(
                embedding=inference_pb2.Embedding(values=embedding.tolist()),
                inference_time_ms=inference_time
            )
            
        except Exception as e:
            logger.error(f"Error in ExtractEmbedding: {e}", exc_info=True)
            context.set_code(grpc.StatusCode.INTERNAL)
            context.set_details(str(e))
            return inference_pb2.EmbeddingResponse()
    
    def CheckLiveness(self, request, context):
        """Check face liveness using multi-stage approach
        
        Strategy:
        1. Try micromovement detection (temporal analysis of natural facial movements)
        2. Try depth sensing if available (IR camera depth data)
        3. Fallback to challenge-response system (nod/turn head movements)
        
        Current challenges available without model change:
        - Head nod (up/down pitch detection via nose Y position)
        - Head turn left/right (yaw detection via nose X position relative to eyes)
        """
        try:
            # TODO: Implement multi-stage liveness detection:
            # 1. Micromovement detection - analyze frame-to-frame landmark shifts
            #    for natural involuntary facial movements (breathing, micro-expressions)
            # 2. Depth sensing - use IR camera depth data if available to detect
            #    3D structure and reject flat photos/screens
            # 3. Challenge-response - if above fail, request explicit head movement
            #    via existing challenge system (detectNod/detectTurn in Go)
            
            # Basic placeholder - always returns true for now
            return inference_pb2.LivenessResponse(
                is_live=True,
                confidence=1.0,
                inference_time_ms=0
            )
        except Exception as e:
            logger.error(f"Error in CheckLiveness: {e}", exc_info=True)
            context.set_code(grpc.StatusCode.INTERNAL)
            context.set_details(str(e))
            return inference_pb2.LivenessResponse()
    
    def Health(self, request, context):
        """Health check"""
        return inference_pb2.HealthResponse(
            healthy=True,
            version=self.version,
            device=self.device,
            models_loaded=["scrfd_face_det_10g", "arcface_r50"]
        )


def serve(host: str = "localhost", port: int = 50051,
          detector_path: str = "../models/det_10g.onnx",
          recognizer_path: str = "../models/arcface_r50.onnx"):
    """Start gRPC server"""
    
    server = grpc.server(futures.ThreadPoolExecutor(max_workers=4))
    
    servicer = InferenceServicer(detector_path, recognizer_path)
    inference_pb2_grpc.add_FaceInferenceServicer_to_server(servicer, server)
    
    address = f"{host}:{port}"
    server.add_insecure_port(address)
    
    logger.info(f"Starting inference service on {address}")
    logger.info(f"Device: {servicer.device}")
    logger.info(f"Detector: {detector_path}")
    logger.info(f"Recognizer: {recognizer_path}")
    
    server.start()
    logger.info("Service ready")
    
    try:
        server.wait_for_termination()
    except KeyboardInterrupt:
        logger.info("Shutting down...")
        server.stop(0)


if __name__ == "__main__":
    import argparse
    
    parser = argparse.ArgumentParser(description="LinuxHello Inference Service")
    parser.add_argument("--host", default="localhost", help="Host to bind to")
    parser.add_argument("--port", type=int, default=50051, help="Port to bind to")
    parser.add_argument("--detector", default="../models/det_10g.onnx",
                       help="Path to face detector model")
    parser.add_argument("--recognizer", default="../models/arcface_r50.onnx",
                       help="Path to face recognizer model")
    
    args = parser.parse_args()
    
    serve(args.host, args.port, args.detector, args.recognizer)
