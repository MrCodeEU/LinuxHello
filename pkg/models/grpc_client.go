// Package models provides face detection and recognition via gRPC
package models

import (
	"bytes"
	"context"
	"fmt"
	"image"
	"image/color"
	"image/jpeg"
	"math"
	"time"

	inference "github.com/facelock/facelock/api"
	"google.golang.org/grpc"
	"google.golang.org/grpc/credentials/insecure"
)

// InferenceClient manages connection to the Python inference service
type InferenceClient struct {
	conn   *grpc.ClientConn
	client inference.FaceInferenceClient
}

// NewInferenceClient creates a new inference client
func NewInferenceClient(address string) (*InferenceClient, error) {
	// Set up connection
	// ctx is no longer needed for NewClient
	// cancel is not needed either, but we might want to keep the timeout logic for the health check?
	// The original code used ctx for DialContext.

	conn, err := grpc.NewClient(address,
		grpc.WithTransportCredentials(insecure.NewCredentials()),
	)
	if err != nil {
		return nil, fmt.Errorf("failed to create client for inference service at %s: %w", address, err)
	}

	client := inference.NewFaceInferenceClient(conn)

	// Check health with timeout
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	healthResp, err := client.Health(ctx, &inference.HealthRequest{})
	if err != nil {
		_ = conn.Close()
		return nil, fmt.Errorf("health check failed: %w", err)
	}

	if !healthResp.Healthy {
		_ = conn.Close()
		return nil, fmt.Errorf("inference service is not healthy")
	}

	fmt.Printf("Connected to inference service v%s on %s\n", healthResp.Version, healthResp.Device)

	return &InferenceClient{
		conn:   conn,
		client: client,
	}, nil
}

// Close closes the client connection
func (c *InferenceClient) Close() error {
	if c.conn != nil {
		return c.conn.Close()
	}
	return nil
}

// DetectFaces performs face detection on an image
func (c *InferenceClient) DetectFaces(ctx context.Context, req *inference.DetectRequest) (*inference.DetectResponse, error) {
	return c.client.DetectFaces(ctx, req)
}

// ExtractEmbedding extracts face embedding from an image with detected face
func (c *InferenceClient) ExtractEmbedding(ctx context.Context, req *inference.EmbeddingRequest) (*inference.EmbeddingResponse, error) {
	return c.client.ExtractEmbedding(ctx, req)
}

// CheckLiveness performs liveness detection on an image with detected face
func (c *InferenceClient) CheckLiveness(ctx context.Context, req *inference.LivenessRequest) (*inference.LivenessResponse, error) {
	return c.client.CheckLiveness(ctx, req)
}

// Detection represents a detected face
type Detection struct {
	X1         float32
	Y1         float32
	X2         float32
	Y2         float32
	Confidence float32
	Landmarks  [][2]float32 // 5-point facial landmarks
}

// FaceDetector provides face detection via gRPC
type FaceDetector struct {
	client       *InferenceClient
	confidence   float32
	nmsThreshold float32
}

// NewFaceDetector creates a new face detector (gRPC-based)
func NewFaceDetector(modelPath string, confidence, nmsThreshold float32, inputSize int) (*FaceDetector, error) {
	// Model path is ignored - we use gRPC service instead
	// For compatibility, we just store the confidence threshold
	return &FaceDetector{
		client:       nil, // Will be set by SetInferenceClient
		confidence:   confidence,
		nmsThreshold: nmsThreshold,
	}, nil
}

// SetInferenceClient sets the gRPC client for this detector
func (fd *FaceDetector) SetInferenceClient(client *InferenceClient) {
	fd.client = client
}

// Detect performs face detection on preprocessed image data
func (fd *FaceDetector) Detect(imageData []float32, imgWidth, imgHeight int) ([]Detection, error) {
	if fd.client == nil {
		return nil, fmt.Errorf("inference client not set")
	}

	// Convert float32 array to image bytes
	// imageData is in [1, 3, H, W] format, we need to convert it
	img := image.NewRGBA(image.Rect(0, 0, imgWidth, imgHeight))

	// Convert CHW to HWC format
	for y := 0; y < imgHeight; y++ {
		for x := 0; x < imgWidth; x++ {
			offset := y*imgWidth + x
			r := uint8(imageData[offset] * 255)
			g := uint8(imageData[imgWidth*imgHeight+offset] * 255)
			b := uint8(imageData[2*imgWidth*imgHeight+offset] * 255)
			img.SetRGBA(x, y, color.RGBA{R: r, G: g, B: b, A: 255})
		}
	}

	// Encode as JPEG
	var buf bytes.Buffer
	if err := jpeg.Encode(&buf, img, &jpeg.Options{Quality: 90}); err != nil {
		return nil, fmt.Errorf("failed to encode image: %w", err)
	}

	// Call gRPC service
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()

	resp, err := fd.client.client.DetectFaces(ctx, &inference.DetectRequest{
		Image: &inference.Image{
			Data:   buf.Bytes(),
			Width:  int32(imgWidth),
			Height: int32(imgHeight),
			Format: "jpeg",
		},
		ConfidenceThreshold: fd.confidence,
		NmsThreshold:        fd.nmsThreshold,
	})
	if err != nil {
		return nil, fmt.Errorf("detection failed: %w", err)
	}

	// Convert protobuf detections to local format
	detections := make([]Detection, 0, len(resp.Detections))
	for _, d := range resp.Detections {
		landmarks := make([][2]float32, len(d.Landmarks))
		for i, lm := range d.Landmarks {
			landmarks[i] = [2]float32{lm.X, lm.Y}
		}

		detections = append(detections, Detection{
			X1:         d.X1,
			Y1:         d.Y1,
			X2:         d.X2,
			Y2:         d.Y2,
			Confidence: d.Confidence,
			Landmarks:  landmarks,
		})
	}

	return detections, nil
}

// Close releases the detector resources (no-op for gRPC client)
func (fd *FaceDetector) Close() error {
	// Client is managed externally
	return nil
}

// FaceRecognizer wraps face recognition via gRPC
type FaceRecognizer struct {
	client    *InferenceClient
	inputSize int
}

// NewFaceRecognizer creates a new face recognizer (gRPC-based)
func NewFaceRecognizer(modelPath string, inputSize int) (*FaceRecognizer, error) {
	// Model path is ignored - we use gRPC service instead
	return &FaceRecognizer{
		client:    nil, // Will be set by SetInferenceClient
		inputSize: inputSize,
	}, nil
}

// SetInferenceClient sets the gRPC client for this recognizer
func (fr *FaceRecognizer) SetInferenceClient(client *InferenceClient) {
	fr.client = client
}

// Recognize extracts face embedding from preprocessed face image
func (fr *FaceRecognizer) Recognize(faceData []float32) ([]float32, error) {
	if fr.client == nil {
		return nil, fmt.Errorf("inference client not set")
	}

	// This function is typically called after face detection
	// We need the original image + detection to align the face
	// For now, return error - this should be called via RecognizeFromImage
	return nil, fmt.Errorf("use RecognizeFromImage instead")
}

// RecognizeFromImage extracts face embedding from image with detected face
func (fr *FaceRecognizer) RecognizeFromImage(img image.Image, detection Detection) ([]float32, error) {
	if fr.client == nil {
		return nil, fmt.Errorf("inference client not set")
	}

	// Encode image
	var buf bytes.Buffer
	if err := jpeg.Encode(&buf, img, &jpeg.Options{Quality: 90}); err != nil {
		return nil, fmt.Errorf("failed to encode image: %w", err)
	}

	// Convert detection to protobuf
	landmarks := make([]*inference.Landmark, len(detection.Landmarks))
	for i, lm := range detection.Landmarks {
		landmarks[i] = &inference.Landmark{X: lm[0], Y: lm[1]}
	}

	pbDetection := &inference.Detection{
		X1:         detection.X1,
		Y1:         detection.Y1,
		X2:         detection.X2,
		Y2:         detection.Y2,
		Confidence: detection.Confidence,
		Landmarks:  landmarks,
	}

	// Call gRPC service
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()

	bounds := img.Bounds()
	resp, err := fr.client.client.ExtractEmbedding(ctx, &inference.EmbeddingRequest{
		Image: &inference.Image{
			Data:   buf.Bytes(),
			Width:  int32(bounds.Dx()),
			Height: int32(bounds.Dy()),
			Format: "jpeg",
		},
		Face: pbDetection,
	})
	if err != nil {
		return nil, fmt.Errorf("embedding extraction failed: %w", err)
	}

	return resp.Embedding.Values, nil
}

// Close releases the recognizer resources (no-op for gRPC client)
func (fr *FaceRecognizer) Close() error {
	// Client is managed externally
	return nil
}

// GetInputSize returns the expected input image size
func (fr *FaceRecognizer) GetInputSize() int {
	return fr.inputSize
}

// DepthLivenessDetector wraps depth liveness detection via gRPC
type DepthLivenessDetector struct {
	client    *InferenceClient
	threshold float32
}

// NewDepthLivenessDetector creates a new depth liveness detector (gRPC-based)
func NewDepthLivenessDetector(modelPath string, threshold float32) (*DepthLivenessDetector, error) {
	// Model path is ignored - we use gRPC service instead
	return &DepthLivenessDetector{
		client:    nil, // Will be set by SetInferenceClient
		threshold: threshold,
	}, nil
}

// SetInferenceClient sets the gRPC client for this detector
func (dld *DepthLivenessDetector) SetInferenceClient(client *InferenceClient) {
	dld.client = client
}

// CheckLiveness performs liveness detection on an image
func (dld *DepthLivenessDetector) CheckLiveness(img image.Image, detection Detection) (bool, float32, error) {
	if dld.client == nil {
		return false, 0, fmt.Errorf("inference client not set")
	}

	// Encode image
	var buf bytes.Buffer
	if err := jpeg.Encode(&buf, img, &jpeg.Options{Quality: 90}); err != nil {
		return false, 0, fmt.Errorf("failed to encode image: %w", err)
	}

	// Convert detection to protobuf
	landmarks := make([]*inference.Landmark, len(detection.Landmarks))
	for i, lm := range detection.Landmarks {
		landmarks[i] = &inference.Landmark{X: lm[0], Y: lm[1]}
	}

	pbDetection := &inference.Detection{
		X1:         detection.X1,
		Y1:         detection.Y1,
		X2:         detection.X2,
		Y2:         detection.Y2,
		Confidence: detection.Confidence,
		Landmarks:  landmarks,
	}

	// Call gRPC service
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()

	bounds := img.Bounds()
	resp, err := dld.client.client.CheckLiveness(ctx, &inference.LivenessRequest{
		Image: &inference.Image{
			Data:   buf.Bytes(),
			Width:  int32(bounds.Dx()),
			Height: int32(bounds.Dy()),
			Format: "jpeg",
		},
		Face: pbDetection,
	})
	if err != nil {
		return false, 0, fmt.Errorf("liveness check failed: %w", err)
	}

	return resp.IsLive, resp.Confidence, nil
}

// Close releases the detector resources (no-op for gRPC client)
func (dld *DepthLivenessDetector) Close() error {
	// Client is managed externally
	return nil
}

// Utility functions

// CosineSimilarity computes cosine similarity between two embeddings
func CosineSimilarity(a, b []float32) float64 {
	if len(a) != len(b) {
		return 0
	}

	var dotProduct, normA, normB float64
	for i := 0; i < len(a); i++ {
		dotProduct += float64(a[i]) * float64(b[i])
		normA += float64(a[i]) * float64(a[i])
		normB += float64(b[i]) * float64(b[i])
	}

	if normA == 0 || normB == 0 {
		return 0
	}

	return dotProduct / (math.Sqrt(normA) * math.Sqrt(normB))
}
