package main

import (
	"bytes"
	"context"
	"encoding/base64"
	"encoding/json"
	"fmt"
	"image"
	"image/color"
	"image/draw"
	"image/jpeg"
	"os"
	"os/exec"
	"regexp"
	"strconv"
	"strings"
	"sync"
	"time"

	"github.com/MrCodeEU/LinuxHello/internal/auth"
	"github.com/MrCodeEU/LinuxHello/internal/config"
	models "github.com/MrCodeEU/LinuxHello/pkg/models"
	"github.com/sirupsen/logrus"
	"github.com/wailsapp/wails/v2/pkg/runtime"
)

// Constants for commonly used strings
const (
	errEngineNotInitialized = "engine not initialized"
	svcLinuxHelloInference  = "linuxhello-inference"
	pathLinuxHelloPAM       = "/usr/bin/linuxhello-pam"
	pathLocalLinuxHelloPAM  = "/usr/local/bin/linuxhello-pam"
	pathScriptLinuxHelloPAM = "./scripts/linuxhello-pam"
)

// App struct for Wails application
type App struct {
	ctx    context.Context
	engine *auth.Engine
	logger *logrus.Logger
	cfg    *config.Config

	// State
	mu            sync.RWMutex
	isEnrolling   bool
	enrollTarget  string
	enrollSamples [][]float32
	enrollMessage string
	cameraRunning bool
	isTestingAuth bool

	// Camera streaming
	streamCtx    context.Context
	streamCancel context.CancelFunc
	streamMu     sync.Mutex
}

// NewApp creates a new App instance
func NewApp() *App {
	return &App{
		logger: logrus.New(),
	}
}

// startup is called when the app starts
func (a *App) startup(ctx context.Context) {
	a.ctx = ctx

	// Load configuration
	var err error
	a.cfg, err = config.Load("")
	if err != nil {
		a.logger.Warnf("Failed to load config, using defaults: %v", err)
		a.cfg = config.DefaultConfig()
	}

	// Set log level
	if level, err := logrus.ParseLevel(a.cfg.Logging.Level); err == nil {
		a.logger.SetLevel(level)
	} else {
		a.logger.SetLevel(logrus.DebugLevel)
	}

	// Create auth engine
	a.engine, err = auth.NewEngine(a.cfg, a.logger)
	if err != nil {
		a.logger.Errorf("Failed to create auth engine: %v", err)
		// Notify frontend so it can show an error state
		go func() {
			time.Sleep(500 * time.Millisecond) // wait for frontend to be ready
			runtime.EventsEmit(a.ctx, "app:error", fmt.Sprintf("Failed to initialize auth engine: %v", err))
		}()
		return
	}

	// Initialize camera
	if err := a.engine.InitializeCamera(); err != nil {
		a.logger.Errorf("Failed to initialize camera: %v", err)
	}
}

// shutdown is called when the app is closing
func (a *App) shutdown(ctx context.Context) {
	if a.streamCancel != nil {
		a.streamCancel()
	}
	if a.engine != nil {
		if err := a.engine.Close(); err != nil {
			a.logger.WithError(err).Error("Failed to close engine")
		}
	}
}

// Response types for frontend

// UserResponse represents a user for the frontend
type UserResponse struct {
	Username string `json:"username"`
	Samples  int    `json:"samples"`
	Active   bool   `json:"active"`
}

// EnrollmentStatus represents enrollment progress
type EnrollmentStatus struct {
	IsEnrolling bool   `json:"is_enrolling"`
	Username    string `json:"username"`
	Progress    int    `json:"progress"`
	Total       int    `json:"total"`
	Message     string `json:"message"`
}

// AuthTestResult represents authentication test result
type AuthTestResult struct {
	Success              bool                    `json:"success"`
	Error                string                  `json:"error,omitempty"`
	User                 string                  `json:"user,omitempty"`
	Confidence           float64                 `json:"confidence,omitempty"`
	ProcessingTime       string                  `json:"processing_time,omitempty"`
	LivenessPassed       bool                    `json:"liveness_passed"`
	ChallengeDescription string                  `json:"challenge_description,omitempty"`
	ImageData            string                  `json:"image_data,omitempty"`
	ImageWidth           int                     `json:"image_width,omitempty"`
	ImageHeight          int                     `json:"image_height,omitempty"`
	BoundingBoxes        []auth.DebugBoundingBox `json:"bounding_boxes,omitempty"`
	FacesDetected        int                     `json:"faces_detected"`
}

// ServiceInfo represents systemd service information
type ServiceInfo struct {
	Status  string `json:"status"`
	Enabled string `json:"enabled"`
}

// LogEntry represents a log entry
type LogEntry struct {
	Timestamp string `json:"timestamp"`
	Level     string `json:"level"`
	Message   string `json:"message"`
	Component string `json:"component,omitempty"`
}

// PAMServiceStatus represents the status of a PAM service
type PAMServiceStatus struct {
	ID         string `json:"id"`
	Name       string `json:"name"`
	PAMFile    string `json:"pamFile"`
	Status     string `json:"status"` // "enabled", "disabled", "not installed"
	ModulePath string `json:"modulePath"`
}

// User management bindings

// GetUsers returns all enrolled users
func (a *App) GetUsers() ([]UserResponse, error) {
	if a.engine == nil {
		return nil, fmt.Errorf(errEngineNotInitialized)
	}

	users, err := a.engine.ListUsers()
	if err != nil {
		return nil, err
	}

	resp := make([]UserResponse, 0, len(users))
	for _, u := range users {
		resp = append(resp, UserResponse{
			Username: u.Username,
			Samples:  len(u.Embeddings),
			Active:   u.Active,
		})
	}
	return resp, nil
}

// DeleteUser deletes a user's enrollment
func (a *App) DeleteUser(username string) error {
	if a.engine == nil {
		return fmt.Errorf(errEngineNotInitialized)
	}
	return a.engine.DeleteUser(username)
}

// Enrollment bindings

// StartEnrollment begins the enrollment process for a user
func (a *App) StartEnrollment(username string) error {
	a.mu.Lock()
	if a.isEnrolling {
		a.mu.Unlock()
		return fmt.Errorf("enrollment already in progress")
	}
	a.isEnrolling = true
	a.enrollTarget = username
	a.enrollSamples = make([][]float32, 0)
	a.enrollMessage = "Looking for face..."
	a.mu.Unlock()

	a.logger.Infof("Enrollment: starting for user %s", username)

	// Ensure camera is running
	if err := a.ensureCameraRunning(); err != nil {
		a.mu.Lock()
		a.isEnrolling = false
		a.mu.Unlock()
		return err
	}

	// Start enrollment processing in background
	go a.processEnrollment()

	return nil
}

// GetEnrollmentStatus returns the current enrollment status
func (a *App) GetEnrollmentStatus() EnrollmentStatus {
	a.mu.RLock()
	defer a.mu.RUnlock()

	message := "Ready for enrollment"
	if a.isEnrolling {
		if len(a.enrollSamples) > 0 {
			message = fmt.Sprintf("Captured %d/%d samples", len(a.enrollSamples), a.cfg.Recognition.EnrollmentSamples)
		} else {
			message = a.enrollMessage
		}
	}

	return EnrollmentStatus{
		IsEnrolling: a.isEnrolling,
		Username:    a.enrollTarget,
		Progress:    len(a.enrollSamples),
		Total:       a.cfg.Recognition.EnrollmentSamples,
		Message:     message,
	}
}

// CancelEnrollment cancels the current enrollment
func (a *App) CancelEnrollment() error {
	a.mu.Lock()
	defer a.mu.Unlock()

	if !a.isEnrolling {
		return fmt.Errorf("no enrollment in progress")
	}

	a.isEnrolling = false
	a.enrollTarget = ""
	a.enrollSamples = nil
	a.enrollMessage = ""
	return nil
}

func (a *App) processEnrollment() {
	ticker := time.NewTicker(500 * time.Millisecond)
	defer ticker.Stop()

	timeout := time.After(30 * time.Second)

	for {
		select {
		case <-timeout:
			a.mu.Lock()
			a.isEnrolling = false
			a.enrollMessage = "Enrollment timed out"
			a.mu.Unlock()
			runtime.EventsEmit(a.ctx, "enrollment:complete", map[string]interface{}{
				"success": false,
				"error":   "Enrollment timed out",
			})
			return
		case <-ticker.C:
			a.mu.RLock()
			enrolling := a.isEnrolling
			a.mu.RUnlock()

			if !enrolling {
				return
			}

			if a.processEnrollFrame() {
				a.mu.RLock()
				done := !a.isEnrolling
				a.mu.RUnlock()

				if done {
					runtime.EventsEmit(a.ctx, "enrollment:complete", map[string]interface{}{
						"success": true,
					})
					return
				}
			}

			// Emit status update
			runtime.EventsEmit(a.ctx, "enrollment:status", a.GetEnrollmentStatus())
		}
	}
}

func (a *App) processEnrollFrame() bool {
	frame, ok := a.engine.GetFrame(true)
	if !ok || frame == nil {
		return false
	}

	img, err := frame.ToImage()
	if err != nil {
		return false
	}

	enhanced := auth.EnhanceImage(img)

	detections, err := a.engine.DetectFaces(enhanced)
	if err != nil {
		a.logger.Warnf("Enrollment: detection error: %v", err)
		return false
	}

	if len(detections) == 0 {
		a.mu.Lock()
		a.enrollMessage = "No face detected - please look at the camera"
		a.mu.Unlock()
		return false
	}

	if len(detections) > 1 {
		a.mu.Lock()
		a.enrollMessage = "Multiple faces detected - ensure only one person is visible"
		a.mu.Unlock()
		return false
	}

	embedding, err := a.engine.ExtractEmbedding(enhanced, detections[0])
	if err != nil {
		a.mu.Lock()
		a.enrollMessage = "Failed to process face - please try again"
		a.mu.Unlock()
		return false
	}

	a.mu.Lock()
	a.enrollSamples = append(a.enrollSamples, embedding)
	a.enrollMessage = fmt.Sprintf("Sample %d/%d captured successfully", len(a.enrollSamples), a.cfg.Recognition.EnrollmentSamples)
	a.logger.Infof("Enrollment: captured sample %d/%d for %s", len(a.enrollSamples), a.cfg.Recognition.EnrollmentSamples, a.enrollTarget)

	if len(a.enrollSamples) >= a.cfg.Recognition.EnrollmentSamples {
		store := a.engine.GetEmbeddingStore()
		_, err := store.GetUser(a.enrollTarget)

		var finalErr error
		if err == nil {
			a.logger.Infof("Enrollment: updating existing user %s", a.enrollTarget)
			finalErr = store.UpdateUser(a.enrollTarget, a.enrollSamples)
		} else {
			a.logger.Infof("Enrollment: creating new user %s", a.enrollTarget)
			_, finalErr = store.CreateUser(a.enrollTarget, a.enrollSamples)
		}

		if finalErr != nil {
			a.logger.Errorf("Enrollment: failed to save to database: %v", finalErr)
			a.enrollMessage = "Failed to save enrollment data"
		} else {
			a.enrollMessage = "Enrollment completed successfully!"
		}

		a.isEnrolling = false
		a.enrollTarget = ""
		a.enrollSamples = nil
	}
	a.mu.Unlock()
	return true
}

// Authentication test bindings

// RunAuthTest performs an authentication test
func (a *App) RunAuthTest() (*AuthTestResult, error) {
	if a.engine == nil {
		return nil, fmt.Errorf(errEngineNotInitialized)
	}

	a.mu.Lock()
	a.isTestingAuth = true
	a.mu.Unlock()

	defer func() {
		a.mu.Lock()
		a.isTestingAuth = false
		a.mu.Unlock()
	}()

	// Ensure camera is running
	if err := a.ensureCameraRunning(); err != nil {
		return &AuthTestResult{Success: false, Error: fmt.Sprintf("Failed to start camera: %v", err)}, nil
	}

	// Give camera time to warm up
	time.Sleep(800 * time.Millisecond)

	// Clear buffer
	for i := 0; i < 5; i++ {
		a.engine.GetFrame(false)
		time.Sleep(50 * time.Millisecond)
	}

	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()

	result, debugInfo, err := a.engine.AuthenticateWithDebug(ctx)
	if err != nil {
		return &AuthTestResult{Success: false, Error: err.Error()}, nil
	}

	response := &AuthTestResult{
		Success:              result.Success,
		LivenessPassed:       result.LivenessPassed,
		ProcessingTime:       result.ProcessingTime.String(),
		ChallengeDescription: result.ChallengeDescription,
		Confidence:           result.Confidence,
	}

	if result.User != nil {
		response.User = result.User.Username
	}

	if result.Error != nil {
		response.Error = result.Error.Error()
	}

	if debugInfo != nil {
		response.ImageData = debugInfo.ImageData
		response.ImageWidth = debugInfo.ImageWidth
		response.ImageHeight = debugInfo.ImageHeight
		response.BoundingBoxes = debugInfo.BoundingBoxes
		response.FacesDetected = len(debugInfo.BoundingBoxes)
	}

	return response, nil
}

// Configuration bindings

// GetConfig returns the current configuration
func (a *App) GetConfig() (*config.Config, error) {
	if a.cfg == nil {
		return nil, fmt.Errorf("configuration not loaded")
	}
	return a.cfg, nil
}

// SaveConfig saves the configuration
func (a *App) SaveConfig(cfg *config.Config) error {
	configPath := "/etc/linuxhello/linuxhello.conf"

	if err := cfg.Save(configPath); err != nil {
		return fmt.Errorf("failed to save configuration to %s: %v", configPath, err)
	}

	a.cfg = cfg

	// Set log level
	if level, err := logrus.ParseLevel(cfg.Logging.Level); err == nil {
		a.logger.SetLevel(level)
	}

	// Re-initialize engine with new config
	a.mu.Lock()
	if a.engine != nil {
		if err := a.engine.Close(); err != nil {
			a.logger.Warnf("Error closing engine: %v", err)
		}
	}
	a.cameraRunning = false

	newEngine, err := auth.NewEngine(cfg, a.logger)
	if err != nil {
		a.engine = nil
		a.mu.Unlock()
		return fmt.Errorf("failed to reinitialize engine with new config: %v", err)
	}
	a.engine = newEngine
	if err := a.engine.InitializeCamera(); err != nil {
		a.logger.Warnf("Failed to initialize camera: %v", err)
	}
	a.mu.Unlock()

	return nil
}

// Camera bindings

// StartCamera starts the camera
func (a *App) StartCamera() error {
	return a.ensureCameraRunning()
}

// StopCamera stops the camera
func (a *App) StopCamera() error {
	// Add panic recovery
	defer func() {
		if r := recover(); r != nil {
			a.logger.Errorf("Panic in StopCamera: %v", r)
			// Try to recover state
			a.cameraRunning = false
			if a.streamCancel != nil {
				a.streamCancel()
				a.streamCancel = nil
			}
		}
	}()

	a.mu.Lock()
	defer a.mu.Unlock()

	if !a.cameraRunning {
		return nil
	}

	if a.streamCancel != nil {
		a.streamCancel()
		a.streamCancel = nil
	}

	if a.engine != nil {
		if err := a.engine.Stop(); err != nil {
			return err
		}
	}
	a.cameraRunning = false
	return nil
}

// StartCameraStream starts streaming camera frames via events
func (a *App) StartCameraStream() error {
	if err := a.ensureCameraRunning(); err != nil {
		return err
	}

	a.streamMu.Lock()
	if a.streamCancel != nil {
		a.streamMu.Unlock()
		return nil // Already streaming
	}

	ctx, cancel := context.WithCancel(context.Background())
	a.streamCtx, a.streamCancel = ctx, cancel
	a.streamMu.Unlock()

	go a.streamCameraFrames(ctx)
	return nil
}

// StopCameraStream stops the camera stream
func (a *App) StopCameraStream() {
	a.streamMu.Lock()
	defer a.streamMu.Unlock()

	if a.streamCancel != nil {
		a.streamCancel()
		a.streamCancel = nil
	}
}

// runFaceDetectionLoop runs face detection at 5 FPS in a separate goroutine
func (a *App) runFaceDetectionLoop(ctx context.Context, ticker *time.Ticker, lastDetections *[]models.Detection, detMu *sync.Mutex) {
	for {
		select {
		case <-ctx.Done():
			return
		case <-ticker.C:
			a.mu.RLock()
			testing := a.isTestingAuth
			a.mu.RUnlock()
			if testing || a.engine == nil {
				continue
			}

			frame, ok := a.engine.GetFrame(true)
			if !ok || frame == nil {
				continue
			}

			img, err := frame.ToImage()
			if err != nil {
				continue
			}

			enhanced := auth.EnhanceImage(img)
			dets, err := a.engine.DetectFaces(enhanced)
			if err == nil {
				detMu.Lock()
				*lastDetections = dets
				detMu.Unlock()
			}
		}
	}
}

// processStreamFrame processes and emits a single camera frame
func (a *App) processStreamFrame(lastDetections []models.Detection) (bool, error) {
	a.mu.RLock()
	testing := a.isTestingAuth
	a.mu.RUnlock()
	if testing {
		return true, nil // Continue but skip frame
	}

	frame, ok := a.engine.GetFrame(true)
	if !ok || frame == nil {
		return false, fmt.Errorf("no frame available")
	}

	img, err := frame.ToImage()
	if err != nil {
		return false, err
	}

	enhanced := auth.EnhanceImage(img)
	frameWithBoxes := drawBoundingBoxes(enhanced, lastDetections)
	base64Frame := a.encodeImageAsBase64(frameWithBoxes)

	if base64Frame != "" {
		runtime.EventsEmit(a.ctx, "camera:frame", base64Frame)
	}

	return true, nil
}

func (a *App) streamCameraFrames(ctx context.Context) {
	streamTicker := time.NewTicker(33 * time.Millisecond)  // ~30 FPS for streaming
	detectTicker := time.NewTicker(200 * time.Millisecond) // ~5 FPS for face detection
	defer streamTicker.Stop()
	defer detectTicker.Stop()

	consecutiveErrors := 0
	const maxConsecutiveErrors = 30 // ~1 second at 30fps

	var lastDetections []models.Detection
	var detMu sync.Mutex

	// Face detection goroutine at 5 FPS
	go a.runFaceDetectionLoop(ctx, detectTicker, &lastDetections, &detMu)

	for {
		select {
		case <-ctx.Done():
			return
		case <-streamTicker.C:
			detMu.Lock()
			dets := lastDetections
			detMu.Unlock()

			ok, err := a.processStreamFrame(dets)
			if err != nil {
				consecutiveErrors++
				if consecutiveErrors >= maxConsecutiveErrors {
					runtime.EventsEmit(a.ctx, "camera:error", "Camera stopped producing frames")
					return
				}
				continue
			}

			if ok {
				consecutiveErrors = 0
			}
		}
	}
}

// clampToImageBounds ensures coordinates are within image boundaries
func clampToImageBounds(x1, y1, x2, y2 int, bounds image.Rectangle) (int, int, int, int) {
	if x1 < 0 {
		x1 = 0
	}
	if y1 < 0 {
		y1 = 0
	}
	if x2 >= bounds.Max.X {
		x2 = bounds.Max.X - 1
	}
	if y2 >= bounds.Max.Y {
		y2 = bounds.Max.Y - 1
	}
	return x1, y1, x2, y2
}

// drawRectangleParams holds parameters for drawing a rectangle
type drawRectangleParams struct {
	rgba      *image.RGBA
	x1, y1    int
	x2, y2    int
	boxColor  color.RGBA
	thickness int
	bounds    image.Rectangle
}

// drawRectangle draws a thick rectangle on the RGBA image
func drawRectangle(params drawRectangleParams) {
	for t := 0; t < params.thickness; t++ {
		// Top and bottom edges
		for x := params.x1; x <= params.x2; x++ {
			if params.y1+t < params.bounds.Max.Y {
				params.rgba.Set(x, params.y1+t, params.boxColor)
			}
			if params.y2-t >= 0 {
				params.rgba.Set(x, params.y2-t, params.boxColor)
			}
		}
		// Left and right edges
		for y := params.y1; y <= params.y2; y++ {
			if params.x1+t < params.bounds.Max.X {
				params.rgba.Set(params.x1+t, y, params.boxColor)
			}
			if params.x2-t >= 0 {
				params.rgba.Set(params.x2-t, y, params.boxColor)
			}
		}
	}
}

// drawBoundingBoxes draws face detection bounding boxes onto the image
func drawBoundingBoxes(img image.Image, detections []models.Detection) image.Image {
	if len(detections) == 0 {
		return img
	}

	bounds := img.Bounds()
	rgba := image.NewRGBA(bounds)
	draw.Draw(rgba, bounds, img, bounds.Min, draw.Src)

	boxColor := color.RGBA{0, 255, 0, 255} // Green
	const thickness = 3

	for _, det := range detections {
		x1, y1, x2, y2 := clampToImageBounds(
			int(det.X1), int(det.Y1), int(det.X2), int(det.Y2), bounds,
		)
		drawRectangle(drawRectangleParams{
			rgba:      rgba,
			x1:        x1,
			y1:        y1,
			x2:        x2,
			y2:        y2,
			boxColor:  boxColor,
			thickness: thickness,
			bounds:    bounds,
		})
	}

	return rgba
}

func (a *App) encodeImageAsBase64(img image.Image) string {
	var buf bytes.Buffer
	if err := jpeg.Encode(&buf, img, &jpeg.Options{Quality: 70}); err != nil {
		return ""
	}
	return base64.StdEncoding.EncodeToString(buf.Bytes())
}

func (a *App) ensureCameraRunning() error {
	a.mu.Lock()
	defer a.mu.Unlock()

	if a.cameraRunning {
		return nil
	}

	if a.engine == nil {
		return fmt.Errorf(errEngineNotInitialized)
	}

	if err := a.engine.Start(); err != nil {
		return err
	}
	a.cameraRunning = true
	return nil
}

// Service management bindings

// GetServiceStatus returns the inference service status.
// Note: systemctl is-active/is-enabled return non-zero for inactive/disabled
// but the output still contains the status string (e.g. "inactive", "disabled").
func (a *App) GetServiceStatus() ServiceInfo {
	out, _ := exec.Command("systemctl", "is-active", svcLinuxHelloInference).CombinedOutput()
	status := strings.TrimSpace(string(out))
	if status == "" {
		status = "unknown"
	}

	out, _ = exec.Command("systemctl", "is-enabled", "linuxhello-inference").CombinedOutput()
	enabled := strings.TrimSpace(string(out))
	if enabled == "" {
		enabled = "unknown"
	}

	return ServiceInfo{
		Status:  status,
		Enabled: enabled,
	}
}

// ControlService controls the systemd service
func (a *App) ControlService(action string) (string, error) {
	var cmd *exec.Cmd

	switch action {
	case "start", "enable":
		if out, err := exec.Command("systemctl", "daemon-reload").CombinedOutput(); err != nil {
			return string(out), fmt.Errorf("daemon-reload failed: %v", err)
		}
		cmd = exec.Command("systemctl", action, svcLinuxHelloInference)
	case "stop", "disable":
		cmd = exec.Command("systemctl", action, svcLinuxHelloInference)
	case "restart":
		if out, err := exec.Command("systemctl", "daemon-reload").CombinedOutput(); err != nil {
			return string(out), fmt.Errorf("daemon-reload failed: %v", err)
		}
		cmd = exec.Command("systemctl", "restart", svcLinuxHelloInference)
	default:
		return "", fmt.Errorf("invalid action: %s", action)
	}

	out, err := cmd.CombinedOutput()
	return string(out), err
}

// PAM bindings

// GetPAMStatus returns the PAM module status
func (a *App) GetPAMStatus() (string, error) {
	script := a.findPAMScript()

	cmd := exec.Command(script, "status")
	out, err := cmd.CombinedOutput()
	if err != nil {
		return "", fmt.Errorf("PAM status check failed: %s (%v)", strings.TrimSpace(string(out)), err)
	}
	return a.stripAnsi(strings.TrimSpace(string(out))), nil
}

// GetPAMServices returns parsed PAM service status
// parsePAMServiceLine parses a single line from the PAM status table
func parsePAMServiceLine(line string) (*PAMServiceStatus, error) {
	fields := strings.Fields(line)
	if len(fields) < 3 {
		return nil, fmt.Errorf("insufficient fields")
	}

	serviceID := fields[0]

	// Handle multi-word status like "not installed"
	var status string
	var pamFileEndIdx int

	if len(fields) >= 2 && fields[len(fields)-2] == "not" && fields[len(fields)-1] == "installed" {
		status = "not installed"
		pamFileEndIdx = len(fields) - 2
	} else {
		status = fields[len(fields)-1]
		pamFileEndIdx = len(fields) - 1
	}

	pamFile := strings.Join(fields[1:pamFileEndIdx], " ")

	return &PAMServiceStatus{
		ID:      serviceID,
		Name:    serviceID,
		PAMFile: pamFile,
		Status:  status,
	}, nil
}

// extractModulePath extracts the PAM module path from a status line
func extractModulePath(line string) string {
	if !strings.Contains(line, "PAM module installed at") {
		return ""
	}
	parts := strings.Split(line, "at ")
	if len(parts) == 2 {
		return strings.TrimSpace(parts[1])
	}
	return ""
}

// isTableStart returns true if the line is the start of the service table
func isTableStart(line string) bool {
	return strings.Contains(line, "SERVICE") && strings.Contains(line, "STATUS")
}

// isTableEnd returns true if the line marks the end of the service table
func isTableEnd(line string) bool {
	return line == "" || strings.Contains(line, "Backups:")
}

// isSeparatorLine returns true if the line is a table separator
func isSeparatorLine(line string) bool {
	return strings.Contains(line, "═") || strings.Contains(line, "─")
}

func (a *App) GetPAMServices() ([]PAMServiceStatus, error) {
	script := a.findPAMScript()

	cmd := exec.Command(script, "status")
	out, err := cmd.CombinedOutput()
	if err != nil {
		return nil, fmt.Errorf("PAM status check failed: %v", err)
	}

	var services []PAMServiceStatus
	var modulePath string
	inTable := false

	lines := strings.Split(string(out), "\n")
	for _, line := range lines {
		line = a.stripAnsi(line)
		line = strings.TrimSpace(line)

		if isTableStart(line) {
			inTable = true
			continue
		}

		if inTable && isTableEnd(line) {
			inTable = false
			continue
		}

		if isSeparatorLine(line) {
			continue
		}

		if path := extractModulePath(line); path != "" {
			modulePath = path
			continue
		}

		if inTable && line != "" {
			service, err := parsePAMServiceLine(line)
			if err == nil {
				services = append(services, *service)
			}
		}
	}

	// Set module path for all services
	for i := range services {
		services[i].ModulePath = modulePath
	}

	return services, nil
}

// PAMAction performs a PAM action
func (a *App) PAMAction(action, service string) (string, error) {
	script := a.findPAMScript()

	args := []string{action}
	if action == "enable" {
		args = append(args, "--yes")
	}
	if service != "" {
		args = append(args, service)
	}

	cmd := exec.Command(script, args...)
	out, err := cmd.CombinedOutput()
	return a.stripAnsi(string(out)), err
}

// PAMToggle enables or disables PAM for sudo
func (a *App) PAMToggle(enable bool) (string, error) {
	script := a.findPAMScript()

	action := "disable"
	if enable {
		action = "enable"
	}

	cmd := exec.Command(script, action, "--yes", "sudo")
	out, err := cmd.CombinedOutput()
	return a.stripAnsi(string(out)), err
}

func (a *App) findPAMScript() string {
	// Prefer linuxhello-pam (supports multiple services)
	if _, err := os.Stat(pathScriptLinuxHelloPAM); err == nil {
		return pathScriptLinuxHelloPAM
	}
	if _, err := os.Stat(pathLinuxHelloPAM); err == nil {
		return pathLinuxHelloPAM
	}
	if _, err := os.Stat(pathLocalLinuxHelloPAM); err == nil {
		return pathLocalLinuxHelloPAM
	}
	// Fallback to old manage-pam.sh (sudo only)
	if _, err := os.Stat("./scripts/manage-pam.sh"); err == nil {
		return "./scripts/manage-pam.sh"
	}
	return pathLinuxHelloPAM
}

var ansiRegex = regexp.MustCompile(`\x1b\[[0-9;]*[a-zA-Z]`)

func (a *App) stripAnsi(str string) string {
	return ansiRegex.ReplaceAllString(str, "")
}

// Logs bindings

// GetLogs returns recent system logs
// parseLogLevel converts journald priority to log level
func parseLogLevel(priority string) string {
	switch priority {
	case "3":
		return "error"
	case "4":
		return "warn"
	case "6":
		return "info"
	case "7":
		return "debug"
	default:
		return "info"
	}
}

// parseJournalLine parses a single JSON line from journalctl output
func parseJournalLine(line string) (*LogEntry, error) {
	var entry struct {
		Timestamp        string `json:"__REALTIME_TIMESTAMP"`
		Message          string `json:"MESSAGE"`
		Priority         string `json:"PRIORITY"`
		SyslogIdentifier string `json:"SYSLOG_IDENTIFIER"`
	}

	if err := json.Unmarshal([]byte(line), &entry); err != nil {
		return nil, err
	}

	if entry.Timestamp == "" {
		return nil, fmt.Errorf("missing timestamp")
	}

	micros, err := strconv.ParseInt(entry.Timestamp, 10, 64)
	if err != nil {
		return nil, err
	}

	timestamp := time.Unix(micros/1000000, (micros%1000000)*1000)
	return &LogEntry{
		Timestamp: timestamp.Format("2006-01-02 15:04:05"),
		Level:     parseLogLevel(entry.Priority),
		Message:   entry.Message,
		Component: entry.SyslogIdentifier,
	}, nil
}

func (a *App) GetLogs(count int) ([]LogEntry, error) {
	if count <= 0 {
		count = 100
	}

	cmd := exec.Command("journalctl", "-u", svcLinuxHelloInference+".service", "--no-pager", "-n", strconv.Itoa(count), "--output", "json")
	output, err := cmd.Output()
	if err != nil {
		return nil, fmt.Errorf("failed to read logs: %v", err)
	}

	var logs []LogEntry
	lines := strings.Split(strings.TrimSpace(string(output)), "\n")

	for _, line := range lines {
		if line == "" {
			continue
		}

		logEntry, err := parseJournalLine(line)
		if err != nil {
			continue
		}
		logs = append(logs, *logEntry)
	}

	// Reverse to show most recent first
	for i, j := 0, len(logs)-1; i < j; i, j = i+1, j-1 {
		logs[i], logs[j] = logs[j], logs[i]
	}

	return logs, nil
}

// DownloadLogs returns comprehensive logs for download
func (a *App) DownloadLogs() (string, error) {
	cmd := exec.Command("journalctl", "-u", svcLinuxHelloInference+".service", "--no-pager", "-n", "1000")
	output, err := cmd.Output()
	if err != nil {
		return "", fmt.Errorf("failed to generate log download: %v", err)
	}
	return string(output), nil
}
