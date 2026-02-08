package auth

import (
	"bytes"
	"context"
	"encoding/base64"
	"fmt"
	"image"
	"image/color"
	"image/draw"
	"image/jpeg"
	"math"
	"os"
	"path/filepath"
	"sync"
	"time"

	inference "github.com/MrCodeEU/LinuxHello/api"
	"github.com/MrCodeEU/LinuxHello/internal/camera"
	"github.com/MrCodeEU/LinuxHello/internal/config"
	"github.com/MrCodeEU/LinuxHello/internal/embedding"
	"github.com/MrCodeEU/LinuxHello/pkg/models"
	"github.com/sirupsen/logrus"
)

const errEncodeImage = "failed to encode image: %w"

// Result represents an authentication result
type Result struct {
	Success              bool
	User                 *embedding.User
	Confidence           float64
	LivenessPassed       bool
	ChallengePassed      bool
	ChallengeDescription string
	Error                error
	ProcessingTime       time.Duration
}

// DebugInfo contains debug information for authentication testing
type DebugInfo struct {
	ImageData     string             `json:"image_data"` // Base64 encoded JPEG
	ImageWidth    int                `json:"image_width"`
	ImageHeight   int                `json:"image_height"`
	BoundingBoxes []DebugBoundingBox `json:"bounding_boxes"`
}

// DebugBoundingBox represents a detected face bounding box
type DebugBoundingBox struct {
	X          int     `json:"x"`
	Y          int     `json:"y"`
	Width      int     `json:"width"`
	Height     int     `json:"height"`
	Confidence float64 `json:"confidence"`
}

// Engine orchestrates the authentication pipeline
type Engine struct {
	config          *config.Config
	logger          *logrus.Logger
	camera          *camera.Camera
	irCamera        *camera.IRCamera
	inferenceClient *models.InferenceClient
	basicLiveness   *LivenessDetector
	embeddingStore  *embedding.Store
	challengeSystem *ChallengeSystem
	failedAttempts  map[string]*FailureTracker
	mu              sync.RWMutex
}

// FailureTracker tracks failed authentication attempts
type FailureTracker struct {
	Count       int
	LastAttempt time.Time
	LockedUntil time.Time
}

// NewEngine creates a new authentication engine
func NewEngine(cfg *config.Config, logger *logrus.Logger) (*Engine, error) {
	// Initialize embedding store
	store, err := embedding.NewStore(cfg.Storage.DatabasePath)
	if err != nil {
		return nil, fmt.Errorf("failed to create embedding store: %w", err)
	}

	// Initialize inference client (gRPC to Python service)
	if cfg.Inference.Address == "" {
		_ = store.Close()
		return nil, fmt.Errorf("inference service address not configured")
	}

	inferenceClient, err := models.NewInferenceClient(cfg.Inference.Address)
	if err != nil {
		_ = store.Close()
		return nil, fmt.Errorf("failed to connect to inference service at %s: %w (is the service running? try: make start-service)", cfg.Inference.Address, err)
	}

	logger.Infof("Connected to inference service v%s on %s", inferenceClient.Version, inferenceClient.Device)

	engine := &Engine{
		config:          cfg,
		logger:          logger,
		inferenceClient: inferenceClient,
		embeddingStore:  store,
		failedAttempts:  make(map[string]*FailureTracker),
		basicLiveness:   NewLivenessDetector(float64(cfg.Liveness.DepthThreshold), 100.0),
	}

	// Initialize challenge system if enabled
	if cfg.Challenge.Enabled {
		engine.challengeSystem = NewChallengeSystem(cfg.Challenge)
	}

	return engine, nil
}

// InitializeCamera initializes the camera for capture
func (e *Engine) InitializeCamera() error {
	// Create camera
	cam, err := camera.NewCamera(e.config.Camera)
	if err != nil {
		return fmt.Errorf("failed to create camera: %w", err)
	}

	// Initialize camera
	if err := cam.Initialize(); err != nil {
		return fmt.Errorf("failed to initialize camera: %w", err)
	}

	e.camera = cam

	// Initialize IR camera if configured
	if e.config.Camera.IRDevice != "" {
		irCfg := e.config.Camera
		irCfg.Device = irCfg.IRDevice

		irCam, err := camera.NewIRCamera(irCfg)
		if err != nil {
			e.logger.Warnf("Failed to create IR camera: %v", err)
		} else {
			if err := irCam.Initialize(); err != nil {
				e.logger.Warnf("Failed to initialize IR camera: %v", err)
			} else {
				e.irCamera = irCam
			}
		}
	}

	return nil
}

// Start starts the camera capture
func (e *Engine) Start() error {
	if e.camera == nil {
		return fmt.Errorf("camera not initialized")
	}

	if err := e.camera.Start(); err != nil {
		return err
	}

	if e.irCamera != nil {
		if err := e.irCamera.Start(); err != nil {
			// If IR camera fails, we should probably stop the main camera and return error
			_ = e.camera.Stop()
			return fmt.Errorf("failed to start IR camera: %w", err)
		}
	}

	// Give camera time to warm up and produce valid frames
	time.Sleep(200 * time.Millisecond)

	return nil
}

// Stop stops the camera capture
func (e *Engine) Stop() error {
	if e.camera != nil {
		if err := e.camera.Stop(); err != nil {
			return err
		}
	}

	if e.irCamera != nil {
		_ = e.irCamera.Stop()
	}

	return nil
}

// Close releases all resources
func (e *Engine) Close() error {
	_ = e.Stop()

	if e.camera != nil {
		_ = e.camera.Close()
	}

	if e.irCamera != nil {
		_ = e.irCamera.Close()
	}

	if e.inferenceClient != nil {
		_ = e.inferenceClient.Close()
	}

	if e.embeddingStore != nil {
		_ = e.embeddingStore.Close()
	}

	return nil
}

// Authenticate performs face authentication against all enrolled users
func (e *Engine) Authenticate(ctx context.Context) (*Result, error) {
	startTime := time.Now()
	result := &Result{Success: false}

	// 1. Capture and Detect
	img, detection, err := e.captureAndDetect()
	if err != nil {
		result.Error = err
		return result, nil
	}

	// 2. Liveness Check
	if err := e.performLivenessCheck(img, detection, result); err != nil {
		result.ProcessingTime = time.Since(startTime)
		return result, nil
	}

	// 3. Challenge-Response
	if err := e.performChallenge(ctx, detection, result); err != nil {
		result.ProcessingTime = time.Since(startTime)
		return result, nil
	}

	// 4. Identification
	if err := e.performIdentification(img, detection, result); err != nil {
		result.ProcessingTime = time.Since(startTime)
		return result, nil
	}

	// Success
	result.Success = true
	result.ProcessingTime = time.Since(startTime)

	e.recordSuccessfulAuth(result)
	e.logger.Infof("Authentication successful for user %s (confidence: %.3f, time: %v)",
		result.User.Username, result.Confidence, result.ProcessingTime)

	return result, nil
}

// performLivenessCheck handles the liveness verification step
func (e *Engine) performLivenessCheck(img image.Image, detection models.Detection, result *Result) error {
	if e.config.Liveness.Enabled {
		passed, err := e.verifyLiveness(img, detection)
		result.LivenessPassed = passed
		if err != nil || !passed {
			if err != nil {
				e.logger.Warnf("Liveness check error: %v", err)
			}
			result.Error = fmt.Errorf("liveness check failed - possible photo or screen")
			return result.Error
		}
	} else {
		result.LivenessPassed = true
	}
	return nil
}

// performChallenge handles the challenge-response step
func (e *Engine) performChallenge(ctx context.Context, detection models.Detection, result *Result) error {
	if e.config.Challenge.Enabled {
		passed, desc, err := e.runChallenge(ctx, detection)
		result.ChallengePassed = passed
		result.ChallengeDescription = desc
		if err != nil {
			e.logger.Warnf("Challenge failed: %v", err)
		}
		if !passed {
			result.Error = fmt.Errorf("challenge-response failed: %s", desc)
			return result.Error
		}
	} else {
		result.ChallengePassed = true
	}
	return nil
}

// performIdentification handles the face identification step
func (e *Engine) performIdentification(img image.Image, detection models.Detection, result *Result) error {
	user, confidence, err := e.identifyFace(img, detection)
	if err != nil {
		if confidence > 0 {
			result.Confidence = confidence
			result.Error = fmt.Errorf("no matching user found (confidence: %.3f)", confidence)
		} else {
			result.Error = fmt.Errorf("identification failed: %w", err)
		}
		return result.Error
	}

	result.User = user
	result.Confidence = confidence
	return nil
}

// recordSuccessfulAuth records a successful authentication
func (e *Engine) recordSuccessfulAuth(result *Result) {
	_ = e.embeddingStore.RecordAuth(
		result.User.ID, result.User.Username, true, result.Confidence,
		result.LivenessPassed, result.ChallengePassed, "",
	)
}

func (e *Engine) captureAndDetect() (image.Image, models.Detection, error) {
	if e.inferenceClient == nil {
		return nil, models.Detection{}, fmt.Errorf("inference client not connected")
	}

	var lastImage image.Image
	maxAttempts := 5 // Reduced from 10 to improve responsiveness

	for attempt := 0; attempt < maxAttempts; attempt++ {
		// Small delay between frame captures to ensure fresh frames
		if attempt > 0 {
			time.Sleep(50 * time.Millisecond)
		}

		// 1. Capture frame
		img, err := e.captureFrameFromCamera(attempt)
		if err != nil {
			// If capture fails, we can't do anything with this attempt
			continue
		}
		lastImage = img

		// 2. Detect faces
		detection, err := e.detectSingleFace(img, attempt)
		if err != nil {
			// Detection failed, try next attempt
			continue
		}

		e.logger.Infof("Successfully captured frame with face detection on attempt %d (confidence: %.3f)",
			attempt+1, detection.Confidence)

		return img, detection, nil
	}

	// All attempts failed
	return lastImage, models.Detection{}, fmt.Errorf("no face detected after %d attempts (ensure face is visible and well-lit)", maxAttempts)
}

// captureFrameFromCamera captures and enhances a frame from the camera
func (e *Engine) captureFrameFromCamera(attempt int) (image.Image, error) {
	frame, ok := e.camera.GetFrame()
	if !ok || frame == nil {
		return nil, fmt.Errorf("failed to capture frame on attempt %d", attempt+1)
	}

	img, err := frame.ToImage()
	if err != nil {
		return nil, fmt.Errorf("failed to convert frame on attempt %d: %w", attempt+1, err)
	}

	// Enhance the image for better detection
	return EnhanceImage(img), nil
}

// detectSingleFace detects faces and ensures exactly one face is found
func (e *Engine) detectSingleFace(img image.Image, attempt int) (models.Detection, error) {
	detections, err := e.DetectFaces(img)
	if err != nil {
		return models.Detection{}, fmt.Errorf("face detection failed on attempt %d: %w", attempt+1, err)
	}

	if len(detections) == 0 {
		return models.Detection{}, fmt.Errorf("no face detected on attempt %d", attempt+1)
	}

	if len(detections) > 1 {
		return models.Detection{}, fmt.Errorf("multiple faces detected on attempt %d (%d faces)", attempt+1, len(detections))
	}

	return detections[0], nil
}

func (e *Engine) verifyLiveness(img image.Image, detection models.Detection) (bool, error) {
	// Try gRPC liveness detector first if available
	if e.inferenceClient != nil {
		livenessPassed, err := e.CheckLiveness(img, detection)
		if err == nil {
			return livenessPassed, nil
		}
		e.logger.Warnf("gRPC liveness check failed: %v", err)
	}

	// Fallback to basic liveness detection
	faceRegion := e.ExtractRegion(img, detection)
	livenessPassed, confidence, err := e.basicLiveness.CheckLiveness(faceRegion)
	if err != nil {
		return false, err
	}
	e.logger.Debugf("Basic liveness: live=%v, confidence=%.3f", livenessPassed, confidence)
	return livenessPassed, nil
}

func (e *Engine) runChallenge(ctx context.Context, detection models.Detection) (bool, string, error) {
	if e.challengeSystem == nil {
		return true, "", nil
	}

	// Generate challenge
	challenge := e.challengeSystem.GenerateChallenge()
	e.logger.Infof("Challenge: %s", challenge.Description)

	// Wait for challenge completion
	timeout := time.Duration(e.config.Challenge.TimeoutSeconds) * time.Second
	timeoutCtx, cancel := context.WithTimeout(ctx, timeout)
	defer cancel()

	// Create detector callback
	detector := func(img image.Image) ([]models.Detection, error) {
		return e.DetectFaces(img)
	}

	completed := e.challengeSystem.WaitForChallenge(timeoutCtx, challenge, e.camera, detection, detector)

	return completed, challenge.Description, nil
}

func (e *Engine) identifyFace(img image.Image, detection models.Detection) (*embedding.User, float64, error) {
	emb, err := e.ExtractEmbedding(img, detection)
	if err != nil {
		return nil, 0, fmt.Errorf("failed to extract embedding: %w", err)
	}

	user, confidence, err := e.embeddingStore.FindBestMatch(
		emb,
		e.config.Recognition.SimilarityThreshold,
	)
	if err != nil {
		return nil, 0, fmt.Errorf("failed to find match: %w", err)
	}

	if user == nil {
		return nil, confidence, fmt.Errorf("user not found")
	}

	return user, confidence, nil
}

// AuthenticateUser authenticates a specific user
func (e *Engine) AuthenticateUser(ctx context.Context, username string) (*Result, error) {
	startTime := time.Now()
	result := &Result{Success: false}

	if err := e.CheckLockout(username); err != nil {
		result.Error = err
		result.ProcessingTime = time.Since(startTime)
		return result, nil
	}

	user, err := e.embeddingStore.GetUser(username)
	if err != nil {
		result.Error = fmt.Errorf("user not found: %w", err)
		return result, nil
	}

	// Reuse captureAndDetect helper
	img, detection, err := e.captureAndDetect()
	if err != nil {
		result.Error = err
		return result, nil
	}

	// Liveness check
	if e.config.Liveness.Enabled && e.inferenceClient != nil {
		livenessPassed, err := e.CheckLiveness(img, detection)
		result.LivenessPassed = livenessPassed
		if err != nil {
			e.logger.Warnf("Liveness check failed: %v", err)
		}
		if !livenessPassed {
			result.Error = fmt.Errorf("liveness check failed")
			result.ProcessingTime = time.Since(startTime)
			_ = e.embeddingStore.RecordAuth(
				user.ID, username, false, 0,
				false, false, "liveness check failed",
			)
			return result, nil
		}
	} else {
		result.LivenessPassed = true
	}

	// Challenge-Response
	if err := e.performChallenge(ctx, detection, result); err != nil {
		result.ProcessingTime = time.Since(startTime)
		_ = e.embeddingStore.RecordAuth(
			user.ID, username, false, 0,
			result.LivenessPassed, false, "challenge failed",
		)
		return result, nil
	}

	// Extract embedding
	embedding, err := e.ExtractEmbedding(img, detection)
	if err != nil {
		result.Error = fmt.Errorf("failed to extract embedding: %w", err)
		return result, nil
	}

	// Compare
	bestScore := -1.0
	for _, userEmbedding := range user.Embeddings {
		score := models.CosineSimilarity(embedding, userEmbedding)
		if score > bestScore {
			bestScore = score
		}
	}

	result.Confidence = bestScore
	result.ProcessingTime = time.Since(startTime)

	if bestScore < e.config.Recognition.SimilarityThreshold {
		result.Error = fmt.Errorf("face does not match (confidence: %.3f)", bestScore)
		_ = e.embeddingStore.RecordAuth(
			user.ID, username, false, bestScore,
			result.LivenessPassed, result.ChallengePassed, "face mismatch",
		)
		return result, nil
	}

	// Success
	result.Success = true
	result.User = user

	_ = e.embeddingStore.RecordAuth(
		user.ID, username, true, bestScore,
		result.LivenessPassed, true, "",
	)

	e.logger.Infof("User %s authenticated successfully (confidence: %.3f, time: %v)",
		username, bestScore, result.ProcessingTime)

	return result, nil
}

// AuthenticateWithDebug performs authentication and returns debug information
func (e *Engine) AuthenticateWithDebug(ctx context.Context) (*Result, *DebugInfo, error) {
	startTime := time.Now()
	result := &Result{Success: false}
	debugInfo := &DebugInfo{}

	// Capture and detect with debug info
	img, detection, err := e.captureAndDetect()

	// Always prepare debug image info
	e.prepareDebugImageInfo(img, debugInfo)

	if err != nil {
		result.Error = err
		result.ProcessingTime = time.Since(startTime)
		return result, debugInfo, nil
	}

	// Add detection debug info
	e.addDetectionDebugInfo(img, detection, debugInfo)

	// Perform authentication steps
	if err := e.performDebugAuthentication(ctx, img, detection, result); err != nil {
		result.ProcessingTime = time.Since(startTime)
		return result, debugInfo, nil
	}

	// Success
	result.Success = true
	result.ProcessingTime = time.Since(startTime)
	e.recordSuccessfulAuth(result)

	e.logger.Infof("Authentication successful for user %s (confidence: %.3f, time: %v)",
		result.User.Username, result.Confidence, result.ProcessingTime)

	return result, debugInfo, nil
}

// prepareDebugImageInfo prepares image data for debug output
func (e *Engine) prepareDebugImageInfo(img image.Image, debugInfo *DebugInfo) {
	if img == nil {
		return
	}

	bounds := img.Bounds()
	debugInfo.ImageWidth = bounds.Dx()
	debugInfo.ImageHeight = bounds.Dy()

	var buf bytes.Buffer
	if err := jpeg.Encode(&buf, img, &jpeg.Options{Quality: 85}); err == nil {
		debugInfo.ImageData = base64.StdEncoding.EncodeToString(buf.Bytes())
	}
}

// addDetectionDebugInfo adds bounding box information to debug info
func (e *Engine) addDetectionDebugInfo(img image.Image, detection models.Detection, debugInfo *DebugInfo) {
	imgBounds := img.Bounds()
	x1 := int(math.Max(0, float64(detection.X1)))
	y1 := int(math.Max(0, float64(detection.Y1)))
	x2 := int(math.Min(float64(imgBounds.Dx()), float64(detection.X2)))
	y2 := int(math.Min(float64(imgBounds.Dy()), float64(detection.Y2)))

	debugInfo.BoundingBoxes = []DebugBoundingBox{
		{
			X:          x1,
			Y:          y1,
			Width:      x2 - x1,
			Height:     y2 - y1,
			Confidence: float64(detection.Confidence),
		},
	}
}

// performDebugAuthentication performs the authentication steps for debug mode
func (e *Engine) performDebugAuthentication(ctx context.Context, img image.Image, detection models.Detection, result *Result) error {
	// Liveness Check
	if err := e.performLivenessCheck(img, detection, result); err != nil {
		return err
	}

	// Challenge-Response
	if err := e.performChallenge(ctx, detection, result); err != nil {
		return err
	}

	// Identification
	return e.performIdentification(img, detection, result)
}

// EnhanceImage converts a grayscale image to RGB for JPEG encoding
// and applies auto-contrast enhancement
func EnhanceImage(img image.Image) *image.RGBA {
	bounds := img.Bounds()
	rgba := image.NewRGBA(bounds)

	// Draw source to RGBA (handles format conversion)
	draw.Draw(rgba, bounds, img, bounds.Min, draw.Src)

	// Calculate luminance statistics
	minY, maxY := calculateLuminanceRange(rgba, bounds)

	// Apply contrast enhancement
	applyContrastEnhancement(rgba, minY, maxY)

	return rgba
}

// calculateLuminanceRange computes the min and max luminance values in the image
func calculateLuminanceRange(rgba *image.RGBA, bounds image.Rectangle) (int, int) {
	minY, maxY := 255, 0
	stride := 4

	for y := bounds.Min.Y; y < bounds.Max.Y; y += stride {
		for x := bounds.Min.X; x < bounds.Max.X; x += stride {
			lum := calculatePixelLuminance(rgba, x, y)
			if lum < minY {
				minY = lum
			}
			if lum > maxY {
				maxY = lum
			}
		}
	}

	return minY, maxY
}

// calculatePixelLuminance calculates the luminance of a pixel at given coordinates
func calculatePixelLuminance(rgba *image.RGBA, x, y int) int {
	off := rgba.PixOffset(x, y)
	r := int(rgba.Pix[off])
	g := int(rgba.Pix[off+1])
	b := int(rgba.Pix[off+2])

	// Simple luminance approximation
	return (r + g + g + b) >> 2
}

// applyContrastEnhancement applies contrast stretching to the image
func applyContrastEnhancement(rgba *image.RGBA, minY, maxY int) {
	// Apply contrast stretching if range is sufficient
	if maxY <= minY+20 {
		return // Skip enhancement if range is too small
	}

	scale := calculateContrastScale(minY, maxY)
	targetMin := 0.0

	enhancePixels(rgba, minY, maxY, scale, targetMin)
}

// calculateContrastScale determines the scaling factor for contrast enhancement
func calculateContrastScale(minY, maxY int) float64 {
	// "Smart" auto-contrast:
	// If image is already quite bright (maxY > 180), be very gentle
	targetMax := 210.0
	if maxY > 180 {
		targetMax = float64(maxY) + 10 // Only slight boost
		if targetMax > 255 {
			targetMax = 255
		}
	}

	scale := (targetMax - 0.0) / float64(maxY-minY)

	// Don't over-amplify (max scale 3.0)
	if scale > 3.0 {
		scale = 3.0
	}

	return scale
}

// enhancePixels applies the contrast enhancement to all pixels in the image
func enhancePixels(rgba *image.RGBA, minY, _ int, scale, targetMin float64) {
	for i := 0; i < len(rgba.Pix); i += 4 {
		// R, G, B channels
		for k := 0; k < 3; k++ {
			val := float64(int(rgba.Pix[i+k]) - minY)
			val = val*scale + targetMin

			// Clamp to valid range
			val = clampValue(val)
			rgba.Pix[i+k] = uint8(val)
		}
	}
}

// clampValue clamps a value to the valid pixel range [0, 255]
func clampValue(val float64) float64 {
	if val < 0 {
		return 0
	}
	if val > 255 {
		return 255
	}
	return val
}

// DetectFaces detects faces in an image using gRPC client
func (e *Engine) DetectFaces(img image.Image) ([]models.Detection, error) {
	if e.inferenceClient == nil {
		return nil, fmt.Errorf("inference client not initialized")
	}

	// Convert to RGB for JPEG encoding (IR cameras output grayscale)
	rgbImg := EnhanceImage(img)

	// Encode image as JPEG
	var buf bytes.Buffer
	if err := jpeg.Encode(&buf, rgbImg, &jpeg.Options{Quality: 90}); err != nil {
		return nil, fmt.Errorf(errEncodeImage, err)
	}

	// Call gRPC service
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()

	bounds := img.Bounds()
	resp, err := e.inferenceClient.DetectFaces(ctx, &inference.DetectRequest{
		Image: &inference.Image{
			Data:   buf.Bytes(),
			Width:  int32(bounds.Dx()),
			Height: int32(bounds.Dy()),
			Format: "jpeg",
		},
		ConfidenceThreshold: e.config.Detection.Confidence,
		NmsThreshold:        e.config.Detection.NMSThreshold,
	})
	if err != nil {
		return nil, fmt.Errorf("detection failed: %w", err)
	}

	// Convert protobuf detections to local format
	detections := make([]models.Detection, 0, len(resp.Detections))
	for _, d := range resp.Detections {
		landmarks := make([][2]float32, len(d.Landmarks))
		for i, lm := range d.Landmarks {
			landmarks[i] = [2]float32{lm.X, lm.Y}
		}

		detections = append(detections, models.Detection{
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

// ExtractEmbedding extracts face embedding using gRPC client
func (e *Engine) ExtractEmbedding(img image.Image, detection models.Detection) ([]float32, error) {
	if e.inferenceClient == nil {
		return nil, fmt.Errorf("inference client not initialized")
	}

	// Convert to RGB for JPEG encoding (IR cameras output grayscale)
	rgbImg := EnhanceImage(img)

	// Encode image as JPEG
	var buf bytes.Buffer
	if err := jpeg.Encode(&buf, rgbImg, &jpeg.Options{Quality: 90}); err != nil {
		return nil, fmt.Errorf(errEncodeImage, err)
	}

	// Convert detection to protobuf format
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
	resp, err := e.inferenceClient.ExtractEmbedding(ctx, &inference.EmbeddingRequest{
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

// CheckLiveness performs liveness detection using gRPC client
func (e *Engine) CheckLiveness(img image.Image, detection models.Detection) (bool, error) {
	if e.inferenceClient == nil {
		return true, nil
	}

	// Convert to RGB for JPEG encoding (IR cameras output grayscale)
	rgbImg := EnhanceImage(img)

	// Encode image as JPEG
	var buf bytes.Buffer
	if err := jpeg.Encode(&buf, rgbImg, &jpeg.Options{Quality: 90}); err != nil {
		return false, fmt.Errorf(errEncodeImage, err)
	}

	// Convert detection to protobuf format
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
	resp, err := e.inferenceClient.CheckLiveness(ctx, &inference.LivenessRequest{
		Image: &inference.Image{
			Data:   buf.Bytes(),
			Width:  int32(bounds.Dx()),
			Height: int32(bounds.Dy()),
			Format: "jpeg",
		},
		Face: pbDetection,
	})
	if err != nil {
		return false, fmt.Errorf("liveness check failed: %w", err)
	}

	e.logger.Debugf("Liveness check: live=%v, confidence=%.3f", resp.IsLive, resp.Confidence)

	return resp.IsLive, nil
}

// ExtractRegion extracts a region from an image
func (e *Engine) ExtractRegion(img image.Image, detection models.Detection) image.Image {
	bounds := img.Bounds()
	x1 := int(detection.X1)
	y1 := int(detection.Y1)
	x2 := int(detection.X2)
	y2 := int(detection.Y2)

	// Clamp
	if x1 < bounds.Min.X {
		x1 = bounds.Min.X
	}
	if y1 < bounds.Min.Y {
		y1 = bounds.Min.Y
	}
	if x2 >= bounds.Max.X {
		x2 = bounds.Max.X - 1
	}
	if y2 >= bounds.Max.Y {
		y2 = bounds.Max.Y - 1
	}

	width := x2 - x1
	height := y2 - y1

	region := image.NewRGBA(image.Rect(0, 0, width, height))

	for y := 0; y < height; y++ {
		for x := 0; x < width; x++ {
			r, g, b, a := img.At(x1+x, y1+y).RGBA()
			region.Set(x, y, color.RGBA{R: uint8(r >> 8), G: uint8(g >> 8), B: uint8(b >> 8), A: uint8(a >> 8)})
		}
	}

	return region
}

// GetFrame returns the next frame from the camera
func (e *Engine) GetFrame(preferIR bool) (*camera.Frame, bool) {
	if preferIR && e.irCamera != nil {
		return e.irCamera.GetFrame()
	}
	if e.camera == nil {
		return nil, false
	}
	return e.camera.GetFrame()
}

// IsStarted returns true if the camera is currently capturing
func (e *Engine) IsStarted() bool {
	// Simple check, in a more complex system we might track state more formally
	return e.camera != nil
}

// TriggerIR attempts to trigger the IR emitter
func (e *Engine) TriggerIR() error {
	if e.camera == nil {
		return fmt.Errorf("camera not initialized")
	}
	return e.camera.TriggerIR()
}

// EnrollUser enrolls a new user
func (e *Engine) EnrollUser(username string, numSamples int, debugDir string) (*embedding.User, error) {
	var embeddings [][]float32

	e.logger.Infof("Starting enrollment for user: %s", username)

	// Initialize enrollment
	if err := e.initializeEnrollment(debugDir); err != nil {
		return nil, err
	}

	// Collect samples
	for i := 0; i < numSamples; i++ {
		embedding, err := e.captureSampleForEnrollment(i, numSamples, debugDir)
		if err != nil {
			return nil, err
		}
		embeddings = append(embeddings, embedding)
	}

	// Create user
	user, err := e.embeddingStore.CreateUser(username, embeddings)
	if err != nil {
		return nil, fmt.Errorf("failed to create user: %w", err)
	}

	e.logger.Infof("User %s enrolled successfully with %d samples", username, numSamples)
	return user, nil
}

// initializeEnrollment prepares the system for user enrollment
func (e *Engine) initializeEnrollment(debugDir string) error {
	// Trigger IR explicitly before enrollment sequence
	if err := e.TriggerIR(); err != nil {
		e.logger.Debugf("Manual IR trigger failed: %v", err)
	}

	// Create debug directory if needed
	if debugDir != "" {
		if err := os.MkdirAll(debugDir, 0755); err != nil {
			e.logger.Warnf("Failed to create debug directory: %v", err)
		}
	}

	return nil
}

// captureSampleForEnrollment captures and processes a single enrollment sample
func (e *Engine) captureSampleForEnrollment(sampleNum, totalSamples int, debugDir string) ([]float32, error) {
	// Periodically re-trigger IR to ensure it stays on
	if sampleNum > 0 {
		_ = e.TriggerIR()
	}

	e.logger.Infof("Capturing sample %d/%d...", sampleNum+1, totalSamples)

	// Wait for stable frame
	time.Sleep(500 * time.Millisecond)

	// Capture and enhance frame
	img, err := e.captureAndEnhanceFrame(sampleNum + 1)
	if err != nil {
		return nil, err
	}

	// Save debug image
	if debugDir != "" {
		e.saveDebugImage(img, debugDir, sampleNum+1, "sample")
	}

	// Detect faces with retry logic
	detections, enhancedImg, err := e.detectFaceWithRetry(img, sampleNum+1, debugDir)
	if err != nil {
		return nil, err
	}

	// Extract embedding
	embedding, err := e.ExtractEmbedding(enhancedImg, detections[0])
	if err != nil {
		return nil, fmt.Errorf("failed to extract embedding from sample %d: %w", sampleNum+1, err)
	}

	return embedding, nil
}

// captureAndEnhanceFrame captures and enhances a single frame
func (e *Engine) captureAndEnhanceFrame(sampleNum int) (image.Image, error) {
	frame, ok := e.camera.GetFrame()
	if !ok {
		return nil, fmt.Errorf("failed to capture frame %d", sampleNum)
	}

	img, err := frame.ToImage()
	if err != nil {
		return nil, fmt.Errorf("failed to convert frame %d: %w", sampleNum, err)
	}

	// Enhance image for IR visibility
	return EnhanceImage(img), nil
}

// saveDebugImage saves an image for debugging purposes
func (e *Engine) saveDebugImage(img image.Image, debugDir string, sampleNum int, prefix string) {
	filename := filepath.Join(debugDir, fmt.Sprintf("%s_%d.jpg", prefix, sampleNum))
	f, err := os.Create(filename)
	if err != nil {
		e.logger.Warnf("Failed to create debug image %s: %v", filename, err)
		return
	}
	defer func() {
		if err := f.Close(); err != nil {
			e.logger.Warnf("Failed to close debug image file %s: %v", filename, err)
		}
	}()

	if err := jpeg.Encode(f, img, &jpeg.Options{Quality: 90}); err != nil {
		e.logger.Warnf("Failed to encode debug image %s: %v", filename, err)
		return
	}

	e.logger.Infof("Saved debug image: %s", filename)
}

// detectFaceWithRetry detects faces with retry logic using IR trigger
func (e *Engine) detectFaceWithRetry(img image.Image, sampleNum int, debugDir string) ([]models.Detection, image.Image, error) {
	detections, err := e.DetectFaces(img)
	if err != nil {
		return nil, nil, fmt.Errorf("face detection failed on sample %d: %w", sampleNum, err)
	}

	// If no face detected, try one more time with forced IR trigger
	if len(detections) == 0 {
		e.logger.Warnf("No face detected in sample %d, attempting IR re-trigger and retry...", sampleNum)

		retryImg, retryDetections, err := e.retryFaceDetection(sampleNum, debugDir)
		if err == nil && len(retryDetections) > 0 {
			e.logger.Infof("Recovered face detection after IR re-trigger")
			return retryDetections, retryImg, nil
		}

		return nil, nil, fmt.Errorf("no face detected in sample %d (check camera/IR)", sampleNum)
	}

	if len(detections) > 1 {
		return nil, nil, fmt.Errorf("multiple faces detected in sample %d", sampleNum)
	}

	return detections, img, nil
}

// retryFaceDetection performs a retry attempt for face detection with IR trigger
func (e *Engine) retryFaceDetection(sampleNum int, debugDir string) (image.Image, []models.Detection, error) {
	// Try one more time with forced IR trigger if detection failed
	_ = e.TriggerIR()
	// Increased wait time for emitter to stabilize
	time.Sleep(500 * time.Millisecond)

	// Recapture
	frame2, ok := e.camera.GetFrame()
	if !ok {
		return nil, nil, fmt.Errorf("failed to capture retry frame")
	}

	img2, err := frame2.ToImage()
	if err != nil {
		return nil, nil, fmt.Errorf("failed to convert retry frame: %w", err)
	}

	// Enhance retry frame
	enhancedImg2 := EnhanceImage(img2)

	detections2, err := e.DetectFaces(enhancedImg2)
	if err != nil {
		return nil, nil, err
	}

	// Save recovered debug image
	if debugDir != "" && len(detections2) > 0 {
		e.saveDebugImage(enhancedImg2, debugDir, sampleNum, "sample_recovered")
	}

	return enhancedImg2, detections2, nil
}

// DeleteUser deletes a user
func (e *Engine) DeleteUser(username string) error {
	return e.embeddingStore.DeleteUser(username)
}

// ListUsers returns all enrolled users
func (e *Engine) ListUsers() ([]embedding.User, error) {
	return e.embeddingStore.ListUsers()
}

// GetEmbeddingStore returns the embedding store
func (e *Engine) GetEmbeddingStore() *embedding.Store {
	return e.embeddingStore
}
