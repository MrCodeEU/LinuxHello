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
    level=logging.DEBUG,  # Changed to DEBUG for troubleshooting
    format='%(asctime)s - %(name)s - %(levelname)s - %(message)s'
)
logger = logging.getLogger(__name__)


class FaceDetector:
    """SCRFD face detector"""
    
    def __init__(self, model_path: str, conf_threshold: float = 0.5, nms_threshold: float = 0.4):
        self.conf_threshold = conf_threshold
        self.nms_threshold = nms_threshold
        self.input_size = (640, 640)
        
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
    
    def preprocess(self, image: np.ndarray) -> np.ndarray:
        """Preprocess image for SCRFD model"""
        # SCRFD works best with square images, resize to input_size
        # Pad to square if needed to maintain aspect ratio
        h, w = image.shape[:2]
        size = max(h, w)
        
        # Create square canvas
        canvas = np.zeros((size, size, 3), dtype=np.uint8)
        # Paste image in center
        y_offset = (size - h) // 2
        x_offset = (size - w) // 2
        canvas[y_offset:y_offset+h, x_offset:x_offset+w] = image
        
        # Resize to model input size
        img = cv2.resize(canvas, self.input_size)
        
        # Convert to RGB and normalize to [0, 1]
        img = cv2.cvtColor(img, cv2.COLOR_BGR2RGB)
        img = img.astype(np.float32) / 255.0
        
        # Transpose to CHW format
        img = img.transpose(2, 0, 1)
        
        # Add batch dimension
        img = np.expand_dims(img, axis=0)
        
        return img
    
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
        
        h, w = image.shape[:2]
        
        # Preprocess
        input_data = self.preprocess(image)
        
        # Run inference
        outputs_list = self.session.run(self.output_names, {self.input_name: input_data})
        
        # Robustly map outputs to (score, bbox, kps) tuples by size
        # 1. Classify all outputs by their 2nd dimension (1, 4, 10)
        scores_out = []
        bboxes_out = []
        kps_out = []
        
        for arr in outputs_list:
            # Flatten batch dim if present (1, N, C) -> (N, C)
            if arr.ndim == 3 and arr.shape[0] == 1:
                arr = arr[0]
            
            # Now expected shapes: (N, 1), (N, 4), (N, 10)
            if arr.ndim < 2:
                 # Flattened? (N,)
                 # Assume score?
                 if len(arr.shape) == 1:
                     arr = arr.reshape(-1, 1)

            cols = arr.shape[-1]
            rows = arr.shape[0] # Number of anchors
            
            if cols == 1:
                scores_out.append((rows, arr))
            elif cols == 4:
                bboxes_out.append((rows, arr))
            elif cols == 10:
                kps_out.append((rows, arr))
                
        # 2. Sort each group by number of anchors descending (80x80=6400, ..., 5x5=25)
        scores_out.sort(key=lambda x: x[0], reverse=True)
        bboxes_out.sort(key=lambda x: x[0], reverse=True)
        kps_out.sort(key=lambda x: x[0], reverse=True)
        
        # Verify alignment
        if not (len(scores_out) == len(bboxes_out) == len(kps_out)):
            logger.error(f"Output count mismatch: scores={len(scores_out)}, bboxes={len(bboxes_out)}, kps={len(kps_out)}")
            return []

        detections = []
        
        # SCRFD person_2.5g uses 5 scales
        feat_sizes = [80, 40, 20, 10, 5]
        strides = [8, 16, 32, 64, 128]
        
        # Use the actual number of received scales, in case model differs
        num_scales = len(scores_out)
        
        try:
            for scale_idx in range(num_scales):
                # Get the tensors for this scale (largest to smallest)
                curr_size, score_tensor = scores_out[scale_idx]
                _, bbox_tensor = bboxes_out[scale_idx]
                _, kps_tensor = kps_out[scale_idx]
                
                # Derive stride from feature map size (640 / feat_size)
                # feat_size is sqrt(curr_size)
                feat_size = int(np.sqrt(curr_size))
                stride = 640 // feat_size
                
                # logger.debug(f"Processing scale {scale_idx}: size={curr_size}, stride={stride}")

                scores = score_tensor.reshape(-1)
                bboxes = bbox_tensor
                keypoints = kps_tensor
                
                # Process each anchor point
                # Use numpy vectorization where possible for speed, but loop is fine for now
                
                # Pre-filter by confidence to reduce loop iterations
                keep_indices = np.where(scores >= conf_threshold)[0]
                
                for i in keep_indices:
                    score = float(scores[i])
                    
                    # Get anchor position
                    h_idx = i // feat_size
                    w_idx = i % feat_size
                    
                    # Center point
                    cx = (w_idx + 0.5) * stride
                    cy = (h_idx + 0.5) * stride
                    
                    # Decode bbox
                    bbox_row = bboxes[i] # (4,)
                    dx1, dy1, dx2, dy2 = bbox_row
                    
                    x1 = (cx - dx1 * stride) / 640 * w
                    y1 = (cy - dy1 * stride) / 640 * h
                    x2 = (cx + dx2 * stride) / 640 * w
                    y2 = (cy + dy2 * stride) / 640 * h
                    
                    # Decode landmarks
                    kps_row = keypoints[i] # (10,)
                    landmarks = []
                    for j in range(5):
                        kpx = kps_row[j * 2]
                        kpy = kps_row[j * 2 + 1]
                        lx = (cx + kpx * stride) / 640 * w
                        ly = (cy + kpy * stride) / 640 * h
                        landmarks.append([lx, ly])
                    
                    detections.append({
                        'bbox': [x1, y1, x2, y2],
                        'confidence': score,
                        'landmarks': landmarks
                    })
        except Exception as e:
            import traceback
            traceback.print_exc()
            logger.error(f"Error parsing detections at scale {scale_idx}: {e}")
            raise e


        
        # Apply NMS
        detections = self.nms(detections, nms_threshold)
        
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
        
        logger.info(f"Face recognizer loaded")
    
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
        """Check face liveness (basic implementation)"""
        try:
            # Basic liveness - just return true for now
            # TODO: Implement proper liveness detection
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
            models_loaded=["scrfd_person_2.5g", "arcface_r50"]
        )


def serve(host: str = "localhost", port: int = 50051,
          detector_path: str = "../models/scrfd_person_2.5g.onnx",
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
    parser.add_argument("--detector", default="../models/scrfd_person_2.5g.onnx",
                       help="Path to face detector model")
    parser.add_argument("--recognizer", default="../models/arcface_r50.onnx",
                       help="Path to face recognizer model")
    
    args = parser.parse_args()
    
    serve(args.host, args.port, args.detector, args.recognizer)
