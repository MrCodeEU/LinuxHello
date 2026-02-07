package main

import (
	"bufio"
	"bytes"
	"context"
	"encoding/json"
	"flag"
	"fmt"
	"image"
	"image/color"
	"image/draw"
	"image/jpeg"
	"log"
	"mime/multipart"
	"net/http"
	"net/textproto"
	"os"
	"os/exec"
	"regexp"
	"runtime/debug"
	"strconv"
	"strings"
	"sync"
	"time"

	"github.com/MrCodeEU/LinuxHello/internal/auth"
	"github.com/MrCodeEU/LinuxHello/internal/camera"
	"github.com/MrCodeEU/LinuxHello/internal/config"
	"github.com/MrCodeEU/LinuxHello/pkg/models"
	"github.com/sirupsen/logrus"
)

const (
	pathLinuxHelloPAM = "/usr/bin/linuxhello-pam"
)

var (
	engine *auth.Engine
	logger *logrus.Logger
	cfg    *config.Config

	// Enrollment state
	enrollMu       sync.Mutex
	isEnrolling    bool
	enrollTarget   string
	enrollSamples  [][]float32
	lastEnrollTime time.Time
	enrollMessage  string

	// Auth Test state
	authTestMu    sync.Mutex
	isTestingAuth bool

	// Streaming
	subscribers = make(map[chan []byte]bool)
	subsMu      sync.Mutex
	camMu       sync.Mutex
	isRunning   bool
)

// HTTP constants
const (
	ContentTypeHeader = "Content-Type"
	ApplicationJSON   = "application/json"
	TextPlain         = "text/plain"
)

// HTTP Status constants
const (
	MethodNotAllowed = "Method Not Allowed"
)

// Error message constants
const (
	FailedToWriteResponse = "Failed to write response"
	FailedToStartCamera   = "Failed to start camera: %v"
)

// Service constants
const (
	InferenceServiceName = "linuxhello-inference"
	GUIServiceName       = "linuxhello-gui"
)

func main() {
	defer func() {
		if r := recover(); r != nil {
			msg := fmt.Sprintf("FATAL PANIC RECOVERED: %v\n%s\n", r, debug.Stack())
			fmt.Print(msg)
			// Write to a crash log file
			_ = os.WriteFile("/tmp/linuxhello-gui-crash.log", []byte(msg), 0644)
			os.Exit(1)
		}
	}()

	if os.Geteuid() != 0 {
		fmt.Println("Error: LinuxHello Manager must be run as root/sudo.")
		fmt.Println("Please run: sudo systemctl start facelock-gui")
		os.Exit(1)
	}

	configPath := flag.String("config", "", "Path to configuration file")
	flag.Parse()

	var err error
	logger = logrus.New()
	logger.SetLevel(logrus.DebugLevel) // Enable debug logging by default for troubleshooting

	cfg, err = config.Load(*configPath)
	if err != nil {
		logger.Warnf("Failed to load config, using defaults: %v", err)
		cfg = config.DefaultConfig()
	}

	setLogLevel(cfg.Logging.Level)

	engine, err = auth.NewEngine(cfg, logger)
	if err != nil {
		log.Fatalf("Failed to create engine: %v", err)
	}
	defer func() {
		if err := engine.Close(); err != nil {
			logger.WithError(err).Error("Failed to close engine")
		}
	}()

	if err := engine.InitializeCamera(); err != nil {
		log.Fatalf("Failed to initialize camera: %v", err)
	}

	go broadcaster()

	// Check if we're running from development directory or installed
	webUIDir := "./frontend/dist"
	if _, err := os.Stat(webUIDir); os.IsNotExist(err) {
		// Use installed location
		webUIDir = "/usr/share/linuxhello/frontend"
	}

	http.Handle("/", http.FileServer(http.Dir(webUIDir)))
	http.HandleFunc("/api/stream", handleStream)
	http.HandleFunc("/api/users", handleUsers)
	http.HandleFunc("/api/enroll", handleEnroll)
	http.HandleFunc("/api/enroll/status", handleEnrollStatus)
	http.HandleFunc("/api/config", handleConfig)
	http.HandleFunc("/api/pam", handlePAM)
	http.HandleFunc("/api/pam/manage", handlePAMManage)
	http.HandleFunc("/api/service", handleService)
	http.HandleFunc("/api/logs", handleLogs)
	http.HandleFunc("/api/logs/download", handleLogsDownload)
	http.HandleFunc("/api/authtest", handleAuthTest)
	http.HandleFunc("/api/camera/start", handleCameraStart)
	http.HandleFunc("/api/camera/stop", handleCameraStop)

	fmt.Println("LinuxHello Manager running at http://localhost:8080")
	if err := http.ListenAndServe(":8080", nil); err != nil {
		log.Fatal(err)
	}
}

func setLogLevel(level string) {
	l, err := logrus.ParseLevel(level)
	if err == nil {
		logger.SetLevel(l)
	}
}

func stripAnsi(str string) string {
	re := regexp.MustCompile(`[\x1b\x9b][\[]()#;?]*((?:[a-zA-Z\d]*(?:;[-a-zA-Z\d\/#&.:=?%@~]*)*)?[\x07] | (?:(?:\d{1,4}(?:;\d{0,4})*)?[\dA-PR-TZcf-ntqry=><~]))`)
	return re.ReplaceAllString(str, "")
}

func ensureCameraState() {
	camMu.Lock()
	defer camMu.Unlock()

	subsMu.Lock()
	numSubs := len(subscribers)
	subsMu.Unlock()

	enrollMu.Lock()
	enrolling := isEnrolling
	enrollMu.Unlock()

	shouldBeRunning := numSubs > 0 || enrolling

	if shouldBeRunning && !isRunning {
		logger.Info("Camera: Starting for active session...")
		if err := engine.Start(); err != nil {
			logger.Errorf("Camera: Failed to start: %v", err)
		} else {
			isRunning = true
		}
	} else if !shouldBeRunning && isRunning {
		logger.Info("Camera: Stopping (idle)...")
		_ = engine.Stop()
		isRunning = false
	}
}

func handleConfig(w http.ResponseWriter, r *http.Request) {
	if r.Method == "GET" {
		w.Header().Set(ContentTypeHeader, ApplicationJSON)
		if err := json.NewEncoder(w).Encode(cfg); err != nil {
			logger.WithError(err).Error("Failed to encode config")
			http.Error(w, "Failed to encode config", http.StatusInternalServerError)
		}
		return
	}

	if r.Method == "POST" {
		if err := json.NewDecoder(r.Body).Decode(cfg); err != nil {
			http.Error(w, err.Error(), 400)
			return
		}

		configPath := flag.Lookup("config").Value.String()
		if configPath == "" {
			// Use system config path when running as service
			configPath = "/etc/linuxhello/linuxhello.conf"
		}

		// Try to save to primary location first, fall back to data directory if needed
		if err := cfg.Save(configPath); err != nil {
			logger.WithError(err).Warnf("Failed to save config to %s, trying fallback location", configPath)
			fallbackPath := "/var/lib/linuxhello/linuxhello.conf"
			if err := cfg.Save(fallbackPath); err != nil {
				logger.WithError(err).Errorf("Failed to save config to both %s and %s", configPath, fallbackPath)
				http.Error(w, fmt.Sprintf("Failed to save configuration: %v", err), 500)
				return
			}
			logger.Infof("Configuration saved to fallback location: %s", fallbackPath)
		}

		setLogLevel(cfg.Logging.Level)
		logger.Info("Settings updated, re-initializing engine...")

		camMu.Lock()
		_ = engine.Close()
		isRunning = false

		newEngine, err := auth.NewEngine(cfg, logger)
		if err == nil {
			engine = newEngine
			_ = engine.InitializeCamera()
			logger.Info("Engine re-initialized successfully")
		}
		camMu.Unlock()

		ensureCameraState()
		w.WriteHeader(200)
		return
	}
}

func handlePAM(w http.ResponseWriter, r *http.Request) {
	switch r.Method {
	case "GET":
		handlePAMStatus(w)
	case "POST":
		handlePAMAction(w, r)
	default:
		http.Error(w, MethodNotAllowed, http.StatusMethodNotAllowed)
	}
}

// handlePAMStatus handles GET requests for PAM status
func handlePAMStatus(w http.ResponseWriter) {
	script := findPAMScript("./scripts/manage-pam.sh", pathLinuxHelloPAM)

	cmd := exec.Command(script, "status")
	out, err := cmd.CombinedOutput()

	w.Header().Set(ContentTypeHeader, TextPlain)

	if err != nil {
		if _, writeErr := w.Write([]byte("unknown")); writeErr != nil {
			logger.WithError(writeErr).Error(FailedToWriteResponse)
		}
		return
	}

	if _, err := w.Write([]byte(strings.TrimSpace(string(out)))); err != nil {
		logger.WithError(err).Error(FailedToWriteResponse)
	}
}

// handlePAMAction handles POST requests for PAM actions
func handlePAMAction(w http.ResponseWriter, r *http.Request) {
	req, err := decodePAMRequest(r)
	if err != nil {
		http.Error(w, err.Error(), 400)
		return
	}

	output, err := executePAMAction(req)
	if err != nil {
		http.Error(w, stripAnsi(output), 500)
		return
	}

	if _, err := w.Write([]byte(stripAnsi(output))); err != nil {
		logger.WithError(err).Error(FailedToWriteResponse)
	}
}

// findPAMScript finds the correct PAM script path
func findPAMScript(primaryPath, fallbackPath string) string {
	if _, err := os.Stat(primaryPath); os.IsNotExist(err) {
		return fallbackPath
	}
	return primaryPath
}

// decodePAMRequest decodes the JSON request for PAM actions
func decodePAMRequest(r *http.Request) (struct {
	Action  string `json:"action"`
	Service string `json:"service"`
}, error) {
	var req struct {
		Action  string `json:"action"`
		Service string `json:"service"`
	}
	err := json.NewDecoder(r.Body).Decode(&req)
	return req, err
}

// executePAMAction executes the PAM action command
func executePAMAction(req struct {
	Action  string `json:"action"`
	Service string `json:"service"`
}) (string, error) {
	script := findPAMScript("./scripts/facelock-pam", "/usr/bin/linuxhello-pam")
	args := buildPAMArgs(req)

	cmd := exec.Command(script, args...)
	out, err := cmd.CombinedOutput()
	return string(out), err
}

// buildPAMArgs builds command line arguments for PAM action
func buildPAMArgs(req struct {
	Action  string `json:"action"`
	Service string `json:"service"`
}) []string {
	args := []string{req.Action}

	if req.Action == "enable" {
		args = append(args, "--yes")
	}

	if req.Service != "" {
		args = append(args, req.Service)
	}

	return args
}

// handlePAMManage handles enabling/disabling PAM module for sudo
func handlePAMManage(w http.ResponseWriter, r *http.Request) {
	if r.Method != "POST" {
		http.Error(w, MethodNotAllowed, http.StatusMethodNotAllowed)
		return
	}

	var req struct {
		Action string `json:"action"` // "enable" or "disable"
	}
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		http.Error(w, err.Error(), 400)
		return
	}

	if req.Action != "enable" && req.Action != "disable" {
		http.Error(w, "Invalid action. Must be 'enable' or 'disable'", 400)
		return
	}

	script := "./scripts/manage-pam.sh"
	if _, err := os.Stat(script); os.IsNotExist(err) {
		script = "/usr/bin/linuxhello-pam"
	}

	cmd := exec.Command("sudo", script, req.Action)
	out, err := cmd.CombinedOutput()
	if err != nil {
		logger.WithError(err).Errorf("Failed to %s PAM module: %s", req.Action, string(out))
		http.Error(w, fmt.Sprintf("Failed to %s PAM module: %s", req.Action, string(out)), 500)
		return
	}

	logger.Infof("PAM module %sd successfully", req.Action)
	w.Header().Set(ContentTypeHeader, ApplicationJSON)
	if err := json.NewEncoder(w).Encode(map[string]interface{}{
		"success": true,
		"message": fmt.Sprintf("PAM module %sd successfully", req.Action),
		"output":  string(out),
	}); err != nil {
		logger.WithError(err).Error(FailedToWriteResponse)
	}
}

func runCommandWithStream(w http.ResponseWriter, command string, args ...string) {
	cmd := exec.Command(command, args...)
	cwd, _ := os.Getwd()
	cmd.Dir = cwd

	stdout, _ := cmd.StdoutPipe()
	stderr, _ := cmd.StderrPipe()

	if err := cmd.Start(); err != nil {
		http.Error(w, err.Error(), 500)
		return
	}

	w.Header().Set(ContentTypeHeader, TextPlain)
	w.WriteHeader(http.StatusOK)

	scanner := bufio.NewScanner(stdout)
	for scanner.Scan() {
		if _, err := fmt.Fprintf(w, "%s\n", stripAnsi(scanner.Text())); err != nil {
			logger.WithError(err).Error(FailedToWriteResponse)
		}
		if f, ok := w.(http.Flusher); ok {
			f.Flush()
		}
	}

	errScanner := bufio.NewScanner(stderr)
	for errScanner.Scan() {
		if _, err := fmt.Fprintf(w, "ERROR: %s\n", stripAnsi(errScanner.Text())); err != nil {
			logger.WithError(err).Error(FailedToWriteResponse)
		}
		if f, ok := w.(http.Flusher); ok {
			f.Flush()
		}
	}

	_ = cmd.Wait()
}

func handleService(w http.ResponseWriter, r *http.Request) {
	if r.Method == "GET" {
		cmd := exec.Command("systemctl", "is-active", InferenceServiceName)
		out, _ := cmd.CombinedOutput()
		status := strings.TrimSpace(string(out))

		cmd = exec.Command("systemctl", "is-enabled", InferenceServiceName)
		out, _ = cmd.CombinedOutput()
		enabled := strings.TrimSpace(string(out))

		resp := map[string]string{
			"status":  status,
			"enabled": enabled,
		}
		w.Header().Set(ContentTypeHeader, ApplicationJSON)
		if err := json.NewEncoder(w).Encode(resp); err != nil {
			logger.WithError(err).Error(FailedToWriteResponse)
		}
		return
	}

	if r.Method == "POST" {
		var req struct {
			Action string `json:"action"`
		}
		if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
			http.Error(w, err.Error(), 400)
			return
		}

		switch req.Action {
		case "start", "enable":
			_ = exec.Command("systemctl", "daemon-reload").Run()
			runCommandWithStream(w, "systemctl", req.Action, InferenceServiceName)
		case "stop", "disable":
			runCommandWithStream(w, "systemctl", req.Action, InferenceServiceName)
		case "restart":
			_ = exec.Command("systemctl", "daemon-reload").Run()
			runCommandWithStream(w, "systemctl", "restart", InferenceServiceName)
		default:
			http.Error(w, "Invalid action", 400)
		}
	}
}

func broadcaster() {
	streamTicker := time.NewTicker(33 * time.Millisecond)  // ~30 FPS for streaming
	detectTicker := time.NewTicker(200 * time.Millisecond) // 5 FPS for face detection
	defer streamTicker.Stop()
	defer detectTicker.Stop()

	firstFrameLogged := false
	var lastDetections []models.Detection
	var detectionsMu sync.Mutex

	// Detection goroutine - runs at 5 FPS
	go func() {
		for range detectTicker.C {
			if !shouldProcessFrame() {
				continue
			}

			authTestMu.Lock()
			testing := isTestingAuth
			authTestMu.Unlock()
			if testing {
				continue
			}

			frame, ok := getCameraFrame()
			if !ok {
				continue
			}

			img, err := frame.ToImage()
			if err != nil {
				continue
			}

			enhanced := auth.EnhanceImage(img)

			camMu.Lock()
			if engine != nil {
				detections, err := engine.DetectFaces(enhanced)
				if err == nil {
					detectionsMu.Lock()
					lastDetections = detections
					detectionsMu.Unlock()
				}
			}
			camMu.Unlock()
		}
	}()

	// Streaming goroutine - runs at 30 FPS
	for range streamTicker.C {
		if !shouldProcessFrame() {
			continue
		}

		// If authentication test is running, pause broadcasting to avoid stealing frames
		authTestMu.Lock()
		testing := isTestingAuth
		authTestMu.Unlock()
		if testing {
			continue
		}

		frame, ok := getCameraFrame()
		if !ok {
			continue
		}

		// Log first frame dimensions to help debug coordinate issues
		if !firstFrameLogged {
			logger.Infof("First frame captured: %dx%d, format: %v", frame.Width, frame.Height, frame.Format)
			firstFrameLogged = true
		}

		img, err := frame.ToImage()
		if err != nil {
			continue
		}

		// Also log image dimensions
		if !firstFrameLogged {
			bounds := img.Bounds()
			logger.Infof("Image bounds after ToImage(): %dx%d", bounds.Dx(), bounds.Dy())
		}

		enhanced := auth.EnhanceImage(img)

		// Draw bounding boxes on the frame
		detectionsMu.Lock()
		frameWithBoxes := drawBoundingBoxes(enhanced, lastDetections)
		detectionsMu.Unlock()

		// Process enrollment if active
		processEnrollmentFrame(enhanced)

		// Send frame with bounding boxes to subscribers
		broadcastFrame(frameWithBoxes)
	}
}

// shouldProcessFrame checks if we need to process frames (has subscribers or enrolling)
func shouldProcessFrame() bool {
	subsMu.Lock()
	numSubs := len(subscribers)
	subsMu.Unlock()

	enrollMu.Lock()
	enrolling := isEnrolling
	enrollMu.Unlock()

	authTestMu.Lock()
	testingAuth := isTestingAuth
	authTestMu.Unlock()

	return numSubs > 0 || enrolling || testingAuth
}

// getCameraFrame safely retrieves a frame from the camera
func getCameraFrame() (*camera.Frame, bool) {
	camMu.Lock()
	defer camMu.Unlock()

	if engine == nil {
		return nil, false
	}

	return engine.GetFrame(true)
}

// drawBoundingBoxes draws bounding boxes and confidence scores on the image
func drawBoundingBoxes(img image.Image, detections []models.Detection) image.Image {
	if len(detections) == 0 {
		return img
	}

	// Convert to RGBA for drawing
	bounds := img.Bounds()
	rgba := image.NewRGBA(bounds)
	draw.Draw(rgba, bounds, img, bounds.Min, draw.Src)

	// Define colors
	boxColor := color.RGBA{0, 255, 0, 255}  // Green for bounding boxes
	textBgColor := color.RGBA{0, 0, 0, 180} // Semi-transparent black background

	for _, det := range detections {
		x1 := int(det.X1)
		y1 := int(det.Y1)
		x2 := int(det.X2)
		y2 := int(det.Y2)

		// Ensure coordinates are within bounds
		if x1 < 0 {
			x1 = 0
		}
		if y1 < 0 {
			y1 = 0
		}
		if x2 > bounds.Dx() {
			x2 = bounds.Dx()
		}
		if y2 > bounds.Dy() {
			y2 = bounds.Dy()
		}

		// Draw bounding box (3 pixel thick lines)
		lineWidth := 3
		for i := 0; i < lineWidth; i++ {
			// Top horizontal line
			for x := x1; x <= x2; x++ {
				if y1+i < bounds.Dy() {
					rgba.Set(x, y1+i, boxColor)
				}
			}
			// Bottom horizontal line
			for x := x1; x <= x2; x++ {
				if y2-i >= 0 {
					rgba.Set(x, y2-i, boxColor)
				}
			}
			// Left vertical line
			for y := y1; y <= y2; y++ {
				if x1+i < bounds.Dx() {
					rgba.Set(x1+i, y, boxColor)
				}
			}
			// Right vertical line
			for y := y1; y <= y2; y++ {
				if x2-i >= 0 {
					rgba.Set(x2-i, y, boxColor)
				}
			}
		}

		// Draw confidence text background (simple rectangle)
		confText := fmt.Sprintf("%.1f%%", det.Confidence*100)
		textX := x1 + 5
		textY := y1 - 20
		if textY < 5 {
			textY = y1 + 20 // If too close to top, draw below the box
		}

		// Draw a simple text background rectangle
		bgHeight := 18
		bgWidth := len(confText) * 8
		for y := textY; y < textY+bgHeight && y < bounds.Dy(); y++ {
			for x := textX; x < textX+bgWidth && x < bounds.Dx(); x++ {
				rgba.Set(x, y, textBgColor)
			}
		}

		// Note: For actual text rendering, we'd need a font library like golang.org/x/image/font
		// For now, the bounding box itself is the most important visual feedback
	}

	return rgba
}

// processEnrollmentFrame handles frame processing during enrollment
func processEnrollmentFrame(img image.Image) {
	enrollMu.Lock()
	defer enrollMu.Unlock()

	if isEnrolling && enrollTarget != "" {
		if time.Since(lastEnrollTime) > 500*time.Millisecond {
			lastEnrollTime = time.Now()
			go processEnrollFrame(img)
		}
	}
}

// broadcastFrame sends the frame to all subscribers
func broadcastFrame(img image.Image) {
	var buf bytes.Buffer
	if err := jpeg.Encode(&buf, img, &jpeg.Options{Quality: 70}); err != nil {
		return
	}

	jpegData := buf.Bytes()

	subsMu.Lock()
	defer subsMu.Unlock()

	for ch := range subscribers {
		select {
		case ch <- jpegData:
		default:
		}
	}
}

func processEnrollFrame(img image.Image) bool {
	camMu.Lock()
	if engine == nil {
		camMu.Unlock()
		return false
	}
	// DetectFaces internally calls EnhanceImage again, but it has a threshold
	// to avoid washing out already stretched images.
	detections, err := engine.DetectFaces(img)
	if err != nil {
		logger.Warnf("Enrollment: detection error: %v", err)
		camMu.Unlock()
		return false
	}
	if len(detections) == 0 {
		logger.Info("Enrollment: no face detected in frame")
		enrollMu.Lock()
		enrollMessage = "No face detected - please look at the camera"
		enrollMu.Unlock()
		camMu.Unlock()
		return false
	}
	if len(detections) > 1 {
		logger.Info("Enrollment: multiple faces detected, skipping frame")
		enrollMu.Lock()
		enrollMessage = "Multiple faces detected - ensure only one person is visible"
		enrollMu.Unlock()
		camMu.Unlock()
		return false
	}

	embedding, err := engine.ExtractEmbedding(img, detections[0])
	camMu.Unlock()

	if err != nil {
		logger.Errorf("Enrollment: failed to extract embedding: %v", err)
		enrollMu.Lock()
		enrollMessage = "Failed to process face - please try again"
		enrollMu.Unlock()
		return false
	}

	enrollMu.Lock()
	enrollSamples = append(enrollSamples, embedding)
	enrollMessage = fmt.Sprintf("Sample %d/%d captured successfully", len(enrollSamples), cfg.Recognition.EnrollmentSamples)
	logger.Infof("Enrollment: captured sample %d/%d for %s", len(enrollSamples), cfg.Recognition.EnrollmentSamples, enrollTarget)

	if len(enrollSamples) >= cfg.Recognition.EnrollmentSamples {
		store := engine.GetEmbeddingStore()
		_, err := store.GetUser(enrollTarget)

		var finalErr error
		if err == nil {
			logger.Infof("Enrollment: updating existing user %s", enrollTarget)
			finalErr = store.UpdateUser(enrollTarget, enrollSamples)
		} else {
			logger.Infof("Enrollment: creating new user %s", enrollTarget)
			_, finalErr = store.CreateUser(enrollTarget, enrollSamples)
		}

		if finalErr != nil {
			logger.Errorf("Enrollment: failed to save to database: %v", finalErr)
			enrollMessage = "Failed to save enrollment data"
		} else {
			enrollMessage = "Enrollment completed successfully!"
		}

		isEnrolling = false
		enrollTarget = ""
		enrollSamples = nil
		logger.Info("Enrollment: process complete")
		go ensureCameraState()
	}
	enrollMu.Unlock()
	return true
}

func handleStream(w http.ResponseWriter, r *http.Request) {
	m := multipart.NewWriter(w)
	w.Header().Set(ContentTypeHeader, "multipart/x-mixed-replace; boundary="+m.Boundary())

	ch := make(chan []byte, 1)
	subsMu.Lock()
	subscribers[ch] = true
	subsMu.Unlock()

	logger.Info("Stream: client connected")
	ensureCameraState()

	defer func() {
		subsMu.Lock()
		delete(subscribers, ch)
		subsMu.Unlock()
		logger.Info("Stream: client disconnected")
		go func() {
			time.Sleep(200 * time.Millisecond)
			ensureCameraState()
		}()
	}()

	for {
		select {
		case <-r.Context().Done():
			return
		case data, ok := <-ch:
			if !ok {
				return
			}
			header := make(textproto.MIMEHeader)
			header.Set(ContentTypeHeader, "image/jpeg")
			header.Set("Content-Length", fmt.Sprint(len(data)))
			mw, err := m.CreatePart(header)
			if err != nil {
				return
			}
			if _, err := mw.Write(data); err != nil {
				logger.WithError(err).Error(FailedToWriteResponse)

				return
			}
			if f, ok := w.(http.Flusher); ok {
				f.Flush()
			}
		}
	}
}

func handleUsers(w http.ResponseWriter, r *http.Request) {
	if r.Method == "DELETE" {
		parts := strings.Split(r.URL.Path, "/")
		username := ""
		if len(parts) >= 4 {
			username = parts[3]
		}
		if username != "" {
			if err := engine.DeleteUser(username); err != nil {
				http.Error(w, err.Error(), 500)
				return
			}
			w.WriteHeader(200)
			return
		}
	}

	users, err := engine.ListUsers()
	if err != nil {
		http.Error(w, err.Error(), 500)
		return
	}

	type userResp struct {
		Username string `json:"username"`
		Samples  int    `json:"samples"`
		Active   bool   `json:"active"`
	}
	resp := make([]userResp, 0)
	for _, u := range users {
		resp = append(resp, userResp{Username: u.Username, Samples: len(u.Embeddings), Active: u.Active})
	}
	w.Header().Set(ContentTypeHeader, ApplicationJSON)
	if err := json.NewEncoder(w).Encode(resp); err != nil {
		logger.WithError(err).Error(FailedToWriteResponse)
	}
}

func handleEnroll(w http.ResponseWriter, r *http.Request) {
	if r.Method != "POST" {
		http.Error(w, MethodNotAllowed, http.StatusMethodNotAllowed)
		return
	}

	var req struct {
		Username string `json:"username"`
	}
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		http.Error(w, "Invalid request", 400)
		return
	}

	enrollMu.Lock()
	if isEnrolling {
		enrollMu.Unlock()
		http.Error(w, "Enrollment already in progress", http.StatusConflict)
		return
	}
	isEnrolling = true
	enrollTarget = req.Username
	enrollSamples = make([][]float32, 0)
	lastEnrollTime = time.Now().Add(-1 * time.Second)
	enrollMessage = "Looking for face..."
	enrollMu.Unlock()

	logger.Infof("Enrollment: starting for user %s", req.Username)
	ensureCameraState()

	start := time.Now()
	for time.Since(start) < 30*time.Second {
		time.Sleep(500 * time.Millisecond)
		enrollMu.Lock()
		done := !isEnrolling
		enrollMu.Unlock()
		if done {
			w.WriteHeader(200)
			if _, err := fmt.Fprintf(w, "Success"); err != nil {
				logger.WithError(err).Error(FailedToWriteResponse)
			}
			return
		}
	}

	enrollMu.Lock()
	isEnrolling = false
	enrollMu.Unlock()
	ensureCameraState()
	logger.Error("Enrollment: timed out after 30 seconds")
	http.Error(w, "Enrollment timed out", http.StatusRequestTimeout)
}

// handleEnrollStatus provides real-time enrollment progress updates
func handleEnrollStatus(w http.ResponseWriter, r *http.Request) {
	if r.Method != "GET" {
		http.Error(w, MethodNotAllowed, http.StatusMethodNotAllowed)
		return
	}

	enrollMu.Lock()
	status := struct {
		IsEnrolling bool   `json:"is_enrolling"`
		Username    string `json:"username"`
		Progress    int    `json:"progress"`
		Total       int    `json:"total"`
		Message     string `json:"message"`
	}{
		IsEnrolling: isEnrolling,
		Username:    enrollTarget,
		Progress:    len(enrollSamples),
		Total:       cfg.Recognition.EnrollmentSamples,
	}

	if isEnrolling {
		if len(enrollSamples) == 0 {
			status.Message = enrollMessage
		} else {
			status.Message = fmt.Sprintf("Captured %d/%d samples", len(enrollSamples), cfg.Recognition.EnrollmentSamples)
		}
	} else {
		status.Message = "Ready for enrollment"
	}
	enrollMu.Unlock()

	w.Header().Set(ContentTypeHeader, ApplicationJSON)
	if err := json.NewEncoder(w).Encode(status); err != nil {
		logger.WithError(err).Error(FailedToWriteResponse)
	}
}

func handleAuthTest(w http.ResponseWriter, r *http.Request) {
	if r.Method != "POST" {
		http.Error(w, MethodNotAllowed, http.StatusMethodNotAllowed)
		return
	}

	logger.Info("Auth Test: Starting authentication test")

	// Set testing flag to pause broadcaster
	authTestMu.Lock()
	isTestingAuth = true
	authTestMu.Unlock()

	defer func() {
		authTestMu.Lock()
		isTestingAuth = false
		authTestMu.Unlock()

		// Resume broadcaster immediately (ensure camera state)
		go ensureCameraState()
	}()

	// Prepare camera for authentication test
	if err := ensureAuthTestCamera(); err != nil {
		http.Error(w, fmt.Sprintf(FailedToStartCamera, err), 500)
		return
	}

	// Clear buffer and perform authentication
	clearCameraBuffer()
	response := performAuthenticationTest(r.Context())

	// Send response
	w.Header().Set(ContentTypeHeader, ApplicationJSON)
	if err := json.NewEncoder(w).Encode(response); err != nil {
		logger.WithError(err).Error(FailedToWriteResponse)
	}
}

// ensureAuthTestCamera ensures the camera is running for authentication test
func ensureAuthTestCamera() error {
	camMu.Lock()
	defer camMu.Unlock()

	if !isRunning {
		if err := engine.Start(); err != nil {
			logger.Errorf("Auth Test: "+FailedToStartCamera, err)
			return err
		}
		isRunning = true
	}
	return nil
}

// clearCameraBuffer clears buffered frames before authentication test
func clearCameraBuffer() {
	// Give camera time to warm up and clear any buffered frames
	time.Sleep(800 * time.Millisecond)

	// Clear buffer by reading several frames
	for i := 0; i < 5; i++ {
		frame, ok := engine.GetFrame(false)
		if ok && frame != nil {
			_ = frame // Discard buffered frame
		}
		time.Sleep(50 * time.Millisecond)
	}
}

// performAuthenticationTest executes the authentication test and builds response
func performAuthenticationTest(ctx context.Context) map[string]interface{} {
	result, debugInfo, err := engine.AuthenticateWithDebug(ctx)
	if err != nil {
		logger.Errorf("Auth Test: Authentication failed: %v", err)
		return map[string]interface{}{
			"success": false,
			"error":   err.Error(),
		}
	}

	response := buildAuthTestResponse(result)
	addDebugInfoToResponse(response, debugInfo)

	return response
}

// buildAuthTestResponse creates the basic response structure from authentication result
func buildAuthTestResponse(result *auth.Result) map[string]interface{} {
	response := map[string]interface{}{
		"success":         result.Success,
		"processing_time": result.ProcessingTime.String(),
		"liveness_passed": result.LivenessPassed,
	}

	if result.ChallengeDescription != "" {
		response["challenge_description"] = result.ChallengeDescription
	}

	if result.User != nil {
		response["user"] = result.User.Username
	}

	if result.Confidence > 0 {
		response["confidence"] = result.Confidence
	}

	if result.Error != nil {
		response["error"] = result.Error.Error()
	}

	return response
}

// addDebugInfoToResponse adds debug information to the authentication test response
func addDebugInfoToResponse(response map[string]interface{}, debugInfo *auth.DebugInfo) {
	if debugInfo == nil {
		return
	}

	if debugInfo.ImageData != "" {
		response["image_data"] = debugInfo.ImageData
		response["image_width"] = debugInfo.ImageWidth
		response["image_height"] = debugInfo.ImageHeight
	}

	if len(debugInfo.BoundingBoxes) > 0 {
		response["bounding_boxes"] = debugInfo.BoundingBoxes
		response["faces_detected"] = len(debugInfo.BoundingBoxes)
	} else {
		response["faces_detected"] = 0
	}
}

func handleCameraStart(w http.ResponseWriter, r *http.Request) {
	if r.Method != "POST" {
		http.Error(w, MethodNotAllowed, http.StatusMethodNotAllowed)
		return
	}

	camMu.Lock()
	defer camMu.Unlock()

	if isRunning {
		w.Header().Set(ContentTypeHeader, ApplicationJSON)
		if err := json.NewEncoder(w).Encode(map[string]interface{}{
			"success": true,
			"message": "Camera already running",
		}); err != nil {
			logger.WithError(err).Error(FailedToWriteResponse)
		}
		return
	}

	if err := engine.Start(); err != nil {
		logger.Errorf(FailedToStartCamera, err)
		http.Error(w, fmt.Sprintf(FailedToStartCamera, err), 500)
		return
	}

	isRunning = true
	logger.Info("Camera started successfully")

	w.Header().Set(ContentTypeHeader, ApplicationJSON)
	if err := json.NewEncoder(w).Encode(map[string]interface{}{
		"success": true,
		"message": "Camera started successfully",
	}); err != nil {
		logger.WithError(err).Error(FailedToWriteResponse)
	}
}

func handleCameraStop(w http.ResponseWriter, r *http.Request) {
	if r.Method != "POST" {
		http.Error(w, MethodNotAllowed, http.StatusMethodNotAllowed)
		return
	}

	camMu.Lock()
	defer camMu.Unlock()

	if !isRunning {
		w.Header().Set(ContentTypeHeader, ApplicationJSON)
		if err := json.NewEncoder(w).Encode(map[string]interface{}{
			"success": true,
			"message": "Camera already stopped",
		}); err != nil {
			logger.WithError(err).Error(FailedToWriteResponse)
		}
		return
	}

	if err := engine.Stop(); err != nil {
		logger.Errorf("Failed to stop camera: %v", err)
		http.Error(w, fmt.Sprintf("Failed to stop camera: %v", err), 500)
		return
	}

	isRunning = false
	logger.Info("Camera stopped successfully")

	w.Header().Set(ContentTypeHeader, ApplicationJSON)
	if err := json.NewEncoder(w).Encode(map[string]interface{}{
		"success": true,
		"message": "Camera stopped successfully",
	}); err != nil {
		logger.WithError(err).Error(FailedToWriteResponse)
	}
}

// handleLogs provides system logs as JSON
func handleLogs(w http.ResponseWriter, r *http.Request) {
	if r.Method != "GET" {
		http.Error(w, MethodNotAllowed, http.StatusMethodNotAllowed)
		return
	}

	// Read recent log entries from journalctl for GUI service
	cmd := exec.Command("journalctl", "-u", "linuxhello-gui.service", "--no-pager", "-n", "100", "--output", "json")
	output, err := cmd.Output()
	if err != nil {
		logger.WithError(err).Error("Failed to read logs from journalctl")
		http.Error(w, "Failed to read system logs", 500)
		return
	}

	// Parse journalctl JSON output and convert to our format
	type JournalEntry struct {
		Timestamp        string `json:"__REALTIME_TIMESTAMP"`
		Message          string `json:"MESSAGE"`
		Priority         string `json:"PRIORITY"`
		SyslogIdentifier string `json:"SYSLOG_IDENTIFIER"`
	}

	type LogEntry struct {
		Timestamp string `json:"timestamp"`
		Level     string `json:"level"`
		Message   string `json:"message"`
		Component string `json:"component,omitempty"`
	}

	var logs []LogEntry
	lines := strings.Split(strings.TrimSpace(string(output)), "\n")

	for _, line := range lines {
		if line == "" {
			continue
		}

		var entry JournalEntry
		if err := json.Unmarshal([]byte(line), &entry); err != nil {
			continue
		}

		// Convert timestamp from microseconds to readable format
		if timestampMicros := entry.Timestamp; timestampMicros != "" {
			if micros, err := strconv.ParseInt(timestampMicros, 10, 64); err == nil {
				timestamp := time.Unix(micros/1000000, (micros%1000000)*1000)

				// Convert priority to level
				level := "info"
				switch entry.Priority {
				case "3":
					level = "error"
				case "4":
					level = "warn"
				case "6":
					level = "info"
				case "7":
					level = "debug"
				}

				logs = append(logs, LogEntry{
					Timestamp: timestamp.Format("2006-01-02 15:04:05"),
					Level:     level,
					Message:   entry.Message,
					Component: entry.SyslogIdentifier,
				})
			}
		}
	}

	// Reverse to show most recent first
	for i, j := 0, len(logs)-1; i < j; i, j = i+1, j-1 {
		logs[i], logs[j] = logs[j], logs[i]
	}

	w.Header().Set(ContentTypeHeader, ApplicationJSON)
	if err := json.NewEncoder(w).Encode(logs); err != nil {
		logger.WithError(err).Error(FailedToWriteResponse)
	}
}

// handleLogsDownload provides logs as downloadable file
func handleLogsDownload(w http.ResponseWriter, r *http.Request) {
	if r.Method != "GET" {
		http.Error(w, MethodNotAllowed, http.StatusMethodNotAllowed)
		return
	}

	// Get comprehensive logs from journalctl
	cmd := exec.Command("journalctl", "-u", "linuxhello-gui.service", "-u", "linuxhello-inference.service", "--no-pager", "-n", "1000")
	output, err := cmd.Output()
	if err != nil {
		logger.WithError(err).Error("Failed to read logs for download")
		http.Error(w, "Failed to generate log download", 500)
		return
	}

	w.Header().Set("Content-Type", "text/plain")
	w.Header().Set("Content-Disposition", fmt.Sprintf("attachment; filename=linuxhello-logs-%s.log", time.Now().Format("2006-01-02")))
	if _, err := w.Write(output); err != nil {
		logger.Warnf("Failed to write logs: %v", err)
	}
}
