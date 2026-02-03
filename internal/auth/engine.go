package auth

import (
	"bytes"
	"context"
	"fmt"
	"image"
	"image/color"
	"image/draw"
	"image/jpeg"
	"os"
	"path/filepath"
	"sync"
	"time"

	inference "github.com/facelock/facelock/api"
	"github.com/facelock/facelock/internal/camera"
	"github.com/facelock/facelock/internal/config"
	"github.com/facelock/facelock/internal/embedding"
	"github.com/facelock/facelock/pkg/models"
	"github.com/sirupsen/logrus"
)

const errEncodeImage = "failed to encode image: %w"

// Result represents an authentication result
type Result struct {
	Success         bool
	User            *embedding.User
	Confidence      float64
	LivenessPassed  bool
	ChallengePassed bool
	Error           error
	ProcessingTime  time.Duration
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

	return e.camera.Start()
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

	// 2. Allow result to carry detection info if needed in future (omitted for now)

	// 3. Liveness Check
	if e.config.Liveness.Enabled {
		passed, err := e.verifyLiveness(img, detection)
		result.LivenessPassed = passed
		if err != nil || !passed {
			if err != nil {
				e.logger.Warnf("Liveness check error: %v", err)
			}
			result.Error = fmt.Errorf("liveness check failed - possible photo or screen")
			result.ProcessingTime = time.Since(startTime)
			return result, nil
		}
	} else {
		result.LivenessPassed = true
	}

	// 4. Challenge-Response
	if e.config.Challenge.Enabled {
		passed, err := e.runChallenge(ctx, detection)
		result.ChallengePassed = passed
		if err != nil {
			e.logger.Warnf("Challenge failed: %v", err)
		}
		if !passed {
			result.Error = fmt.Errorf("challenge-response failed")
			result.ProcessingTime = time.Since(startTime)
			return result, nil
		}
	} else {
		result.ChallengePassed = true
	}

	// 5. Identification
	user, confidence, err := e.identifyFace(img, detection)
	if err != nil {
		if confidence > 0 {
			result.Confidence = confidence
			result.Error = fmt.Errorf("no matching user found (confidence: %.3f)", confidence)
		} else {
			result.Error = fmt.Errorf("identification failed: %w", err)
		}
		result.ProcessingTime = time.Since(startTime)
		return result, nil
	}

	// Success
	result.Success = true
	result.User = user
	result.Confidence = confidence
	result.ProcessingTime = time.Since(startTime)

	_ = e.embeddingStore.RecordAuth(
		user.ID, user.Username, true, confidence,
		result.LivenessPassed, result.ChallengePassed, "",
	)

	e.logger.Infof("Authentication successful for user %s (confidence: %.3f, time: %v)",
		user.Username, confidence, result.ProcessingTime)

	return result, nil
}

func (e *Engine) captureAndDetect() (image.Image, models.Detection, error) {
	if e.inferenceClient == nil {
		return nil, models.Detection{}, fmt.Errorf("inference client not connected")
	}

	frame, ok := e.camera.GetFrame()
	if !ok {
		return nil, models.Detection{}, fmt.Errorf("failed to capture frame")
	}

	img, err := frame.ToImage()
	if err != nil {
		return nil, models.Detection{}, fmt.Errorf("failed to convert frame: %w", err)
	}

	detections, err := e.detectFaces(img)
	if err != nil {
		return nil, models.Detection{}, fmt.Errorf("face detection failed: %w", err)
	}

	if len(detections) == 0 {
		return nil, models.Detection{}, fmt.Errorf("no face detected")
	}

	if len(detections) > 1 {
		return nil, models.Detection{}, fmt.Errorf("multiple faces detected")
	}

	return img, detections[0], nil
}

func (e *Engine) verifyLiveness(img image.Image, detection models.Detection) (bool, error) {
	// Try gRPC liveness detector first if available
	if e.inferenceClient != nil {
		livenessPassed, err := e.checkLiveness(img, detection)
		if err == nil {
			return livenessPassed, nil
		}
		e.logger.Warnf("gRPC liveness check failed: %v", err)
	}

	// Fallback to basic liveness detection
	faceRegion := e.extractRegion(img, detection)
	livenessPassed, confidence, err := e.basicLiveness.CheckLiveness(faceRegion)
	if err != nil {
		return false, err
	}
	e.logger.Debugf("Basic liveness: live=%v, confidence=%.3f", livenessPassed, confidence)
	return livenessPassed, nil
}

func (e *Engine) runChallenge(ctx context.Context, detection models.Detection) (bool, error) {
	if e.challengeSystem == nil {
		return true, nil
	}
	return e.performChallenge(ctx, detection)
}

func (e *Engine) identifyFace(img image.Image, detection models.Detection) (*embedding.User, float64, error) {
	emb, err := e.extractEmbedding(img, detection)
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
		livenessPassed, err := e.checkLiveness(img, detection)
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

	// Extract embedding
	embedding, err := e.extractEmbedding(img, detection)
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
			result.LivenessPassed, false, "face mismatch",
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

// convertToRGB converts a grayscale image to RGB for JPEG encoding
// and applies auto-contrast enhancement
func convertToRGB(img image.Image) *image.RGBA {
	bounds := img.Bounds()
	rgba := image.NewRGBA(bounds)

	// Draw source to RGBA (handles format conversion)
	draw.Draw(rgba, bounds, img, bounds.Min, draw.Src)

	// Calculate min/max luminance for auto-contrast
	minY, maxY := 255, 0

	// Sample center area for statistics (ignore borders which might be noisy)
	// or just scan the whole image (it's small enough typically)
	// For speed, let's scan with a stride
	stride := 4
	for y := bounds.Min.Y; y < bounds.Max.Y; y += stride {
		for x := bounds.Min.X; x < bounds.Max.X; x += stride {
			off := rgba.PixOffset(x, y)
			r := int(rgba.Pix[off])
			g := int(rgba.Pix[off+1])
			b := int(rgba.Pix[off+2])

			// Simple luminance approximation
			lum := (r + g + g + b) >> 2
			if lum < minY {
				minY = lum
			}
			if lum > maxY {
				maxY = lum
			}
		}
	}

	// Apply contrast stretching if range is sufficient
	// "Smart" auto-contrast:
	// 1. If image is dark (max < 210), boost it to ~210.
	// 2. If image is bright (max > 210), keep exposure (don't blow out highlights).
	// 3. Always fix black point (subtract min).
	// This fixes both "Too Dark" (IR) and "Too Bright" (User close to camera) cases.
	if maxY > minY+10 {
		// Determine target max peak
		// We want at least peak brightness of 210, but not more than original if it was already bright
		targetMax := float64(maxY)
		if targetMax < 210.0 {
			targetMax = 210.0
		}
		// Clamp ceiling
		if targetMax > 255.0 {
			targetMax = 255.0
		}

		targetMin := 0.0

		// If the range is very compressed, we might amplify noise, so limit scale?
		// But in dark IR, range is naturally compressed, so we DO want amplification.

		scale := (targetMax - targetMin) / float64(maxY-minY)

		for i := 0; i < len(rgba.Pix); i += 4 {
			// R, G, B
			for k := 0; k < 3; k++ {
				val := float64(int(rgba.Pix[i+k]) - minY)
				val = val*scale + targetMin

				// Clamp
				if val < 0 {
					val = 0
				}
				if val > 255 {
					val = 255
				}
				rgba.Pix[i+k] = uint8(val)
			}
		}
	}

	return rgba
}

// detectFaces detects faces in an image using gRPC client
func (e *Engine) detectFaces(img image.Image) ([]models.Detection, error) {
	if e.inferenceClient == nil {
		return nil, fmt.Errorf("inference client not initialized")
	}

	// Convert to RGB for JPEG encoding (IR cameras output grayscale)
	rgbImg := convertToRGB(img)

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

// extractEmbedding extracts face embedding using gRPC client
func (e *Engine) extractEmbedding(img image.Image, detection models.Detection) ([]float32, error) {
	if e.inferenceClient == nil {
		return nil, fmt.Errorf("inference client not initialized")
	}

	// Convert to RGB for JPEG encoding (IR cameras output grayscale)
	rgbImg := convertToRGB(img)

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

// checkLiveness performs liveness detection using gRPC client
func (e *Engine) checkLiveness(img image.Image, detection models.Detection) (bool, error) {
	if e.inferenceClient == nil {
		return true, nil
	}

	// Convert to RGB for JPEG encoding (IR cameras output grayscale)
	rgbImg := convertToRGB(img)

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

// performChallenge performs challenge-response
func (e *Engine) performChallenge(ctx context.Context, initialDetection models.Detection) (bool, error) {
	if e.challengeSystem == nil {
		return true, nil
	}

	// Generate challenge
	challenge := e.challengeSystem.GenerateChallenge()
	e.logger.Infof("Challenge: %s", challenge.Description)

	// Wait for challenge completion
	timeout := time.Duration(e.config.Challenge.TimeoutSeconds) * time.Second
	timeoutCtx, cancel := context.WithTimeout(ctx, timeout)
	defer cancel()

	completed := e.challengeSystem.WaitForChallenge(timeoutCtx, challenge, e.camera, initialDetection)

	return completed, nil
}

// extractRegion extracts a region from an image
func (e *Engine) extractRegion(img image.Image, detection models.Detection) image.Image {
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

	for i := 0; i < numSamples; i++ {
		// Periodically re-trigger IR to ensure it stays on
		if i > 0 {
			_ = e.TriggerIR()
		}

		e.logger.Infof("Capturing sample %d/%d...", i+1, numSamples)

		// Wait for stable frame
		time.Sleep(500 * time.Millisecond)

		// Capture frame
		frame, ok := e.camera.GetFrame()
		if !ok {
			return nil, fmt.Errorf("failed to capture frame %d", i+1)
		}

		// Convert to image
		img, err := frame.ToImage()
		if err != nil {
			return nil, fmt.Errorf("failed to convert frame %d: %w", i+1, err)
		}

		// Enhance image (Auto-Contrast) for IR visibility
		// We do this here so we can save the debug image exactly as the detector sees it
		enhancedImg := convertToRGB(img)

		// Save debug image
		if debugDir != "" {
			filename := filepath.Join(debugDir, fmt.Sprintf("sample_%d.jpg", i+1))
			f, err := os.Create(filename)
			if err != nil {
				e.logger.Warnf("Failed to create debug image %s: %v", filename, err)
			} else {
				if err := jpeg.Encode(f, enhancedImg, &jpeg.Options{Quality: 90}); err != nil {
					e.logger.Warnf("Failed to encode debug image %s: %v", filename, err)
				}
				_ = f.Close()
				e.logger.Infof("Saved debug image: %s", filename)
			}
		}

		// Detect face using enhanced image
		detections, err := e.detectFaces(enhancedImg)
		if err != nil {
			return nil, fmt.Errorf("face detection failed on sample %d: %w", i+1, err)
		}

		if len(detections) == 0 {
			e.logger.Warnf("No face detected in sample %d, attempting IR re-trigger and retry...", i+1)

			// Try one more time with forced IR trigger if detection failed
			_ = e.TriggerIR()
			// Increased wait time for emitter to stabilize
			time.Sleep(500 * time.Millisecond)

			// Recapture
			if frame2, ok := e.camera.GetFrame(); ok {
				if img2, err := frame2.ToImage(); err == nil {
					// Enhance retry frame
					enhancedImg2 := convertToRGB(img2)

					detections2, err := e.detectFaces(enhancedImg2)
					if err == nil && len(detections2) > 0 {
						e.logger.Infof("Recovered face detection after IR re-trigger")
						enhancedImg = enhancedImg2
						detections = detections2

						// Save recovered debug image
						if debugDir != "" {
							filename := filepath.Join(debugDir, fmt.Sprintf("sample_%d_recovered.jpg", i+1))
							f, err := os.Create(filename)
							if err == nil {
								_ = jpeg.Encode(f, enhancedImg, &jpeg.Options{Quality: 90})
								_ = f.Close()
							}
						}
					}
				}
			}

			if len(detections) == 0 {
				return nil, fmt.Errorf("no face detected in sample %d (check camera/IR)", i+1)
			}
		}

		if len(detections) > 1 {
			return nil, fmt.Errorf("multiple faces detected in sample %d", i+1)
		}

		// Extract embedding using the valid detection and enhanced image
		embedding, err := e.extractEmbedding(enhancedImg, detections[0])
		if err != nil {
			return nil, fmt.Errorf("failed to extract embedding from sample %d: %w", i+1, err)
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
