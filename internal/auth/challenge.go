package auth

import (
	"context"
	"math"
	"math/rand"
	"time"

	"github.com/facelock/facelock/internal/camera"
	"github.com/facelock/facelock/internal/config"
	"github.com/facelock/facelock/pkg/models"
)

// ChallengeType represents the type of challenge
type ChallengeType string

const (
	ChallengeBlink     ChallengeType = "blink"
	ChallengeNod       ChallengeType = "nod"
	ChallengeTurnLeft  ChallengeType = "turn_left"
	ChallengeTurnRight ChallengeType = "turn_right"
	ChallengeSmile     ChallengeType = "smile"
)

// Challenge represents a single challenge
type Challenge struct {
	Type        ChallengeType
	Description string
	Timeout     time.Duration
}

// ChallengeSystem manages challenge-response authentication
type ChallengeSystem struct {
	config         config.ChallengeConfig
	availableTypes []ChallengeType
}

// NewChallengeSystem creates a new challenge system
func NewChallengeSystem(cfg config.ChallengeConfig) *ChallengeSystem {
	cs := &ChallengeSystem{
		config: cfg,
	}

	// Parse challenge types
	for _, t := range cfg.ChallengeTypes {
		switch t {
		case "blink":
			cs.availableTypes = append(cs.availableTypes, ChallengeBlink)
		case "nod":
			cs.availableTypes = append(cs.availableTypes, ChallengeNod)
		case "turn_left":
			cs.availableTypes = append(cs.availableTypes, ChallengeTurnLeft)
		case "turn_right":
			cs.availableTypes = append(cs.availableTypes, ChallengeTurnRight)
		case "smile":
			cs.availableTypes = append(cs.availableTypes, ChallengeSmile)
		}
	}

	return cs
}

// GenerateChallenge creates a random challenge
func (cs *ChallengeSystem) GenerateChallenge() Challenge {
	if len(cs.availableTypes) == 0 {
		return Challenge{
			Type:        ChallengeBlink,
			Description: "Please blink",
			Timeout:     time.Duration(cs.config.TimeoutSeconds) * time.Second,
		}
	}

	challengeType := cs.availableTypes[rand.Intn(len(cs.availableTypes))]

	var description string
	switch challengeType {
	case ChallengeBlink:
		description = "Please blink your eyes"
	case ChallengeNod:
		description = "Please nod your head"
	case ChallengeTurnLeft:
		description = "Please turn your head to the left"
	case ChallengeTurnRight:
		description = "Please turn your head to the right"
	case ChallengeSmile:
		description = "Please smile"
	}

	return Challenge{
		Type:        challengeType,
		Description: description,
		Timeout:     time.Duration(cs.config.TimeoutSeconds) * time.Second,
	}
}

// WaitForChallenge waits for the user to complete the challenge
func (cs *ChallengeSystem) WaitForChallenge(
	ctx context.Context,
	challenge Challenge,
	cam *camera.Camera,
	initialDetection models.Detection,
) bool {
	switch challenge.Type {
	case ChallengeBlink:
		return cs.detectBlink(ctx, cam, initialDetection)
	case ChallengeNod:
		return cs.detectNod(ctx, cam, initialDetection)
	case ChallengeTurnLeft, ChallengeTurnRight:
		return cs.detectTurn(ctx, cam, initialDetection, challenge.Type)
	default:
		return false
	}
}

// detectBlink detects eye blinking
func (cs *ChallengeSystem) detectBlink(
	ctx context.Context,
	cam *camera.Camera,
	initialDetection models.Detection,
) bool {
	// Blink detection using eye landmarks
	// Landmarks: 0=left eye, 1=right eye, 2=nose, 3=left mouth, 4=right mouth

	if len(initialDetection.Landmarks) < 2 {
		return false
	}

	// Track eye state over time
	blinkDetected := false
	framesWithEyesOpen := 0
	framesWithEyesClosed := 0
	consecutiveClosed := 0

	ticker := time.NewTicker(50 * time.Millisecond) // 20 FPS check
	defer ticker.Stop()

	timeout := time.After(time.Duration(cs.config.TimeoutSeconds) * time.Second)

	for {
		select {
		case <-ctx.Done():
			return false
		case <-timeout:
			return blinkDetected
		case <-ticker.C:
			// Get frame
			frame, ok := cam.GetFrame()
			if !ok {
				continue
			}

			img, err := frame.ToImage()
			if err != nil {
				continue
			}

			// Detect faces (simplified - in production use a proper face detector)
			// For now, assume face position is stable

			// Check eye state using simple brightness analysis
			// In production, use proper eye aspect ratio (EAR) calculation
			leftEyeOpen := cs.isEyeOpen(img, initialDetection.Landmarks[0])
			rightEyeOpen := cs.isEyeOpen(img, initialDetection.Landmarks[1])

			if leftEyeOpen && rightEyeOpen {
				framesWithEyesOpen++
				consecutiveClosed = 0
			} else {
				framesWithEyesClosed++
				consecutiveClosed++
			}

			// Detect blink: eyes closed for a short period then open
			if consecutiveClosed >= 3 && framesWithEyesOpen > 5 {
				return true
			}

			// Reset if tracking too long
			if framesWithEyesOpen > 100 {
				framesWithEyesOpen = 0
				framesWithEyesClosed = 0
			}
		}
	}
}

// isEyeOpen checks if an eye is open based on region brightness variance
// In production, use proper EAR (Eye Aspect Ratio) calculation
func (cs *ChallengeSystem) isEyeOpen(img interface{}, landmark [2]float32) bool {
	// Simplified - always return true for PoC
	// In production:
	// 1. Extract eye region around landmark
	// 2. Calculate eye aspect ratio (EAR)
	// 3. EAR < threshold means closed
	return true
}

// detectNod detects head nodding
func (cs *ChallengeSystem) detectNod(
	ctx context.Context,
	cam *camera.Camera,
	initialDetection models.Detection,
) bool {
	// Nod detection using vertical movement of face center
	// Calculate initial position for reference
	initialY := (initialDetection.Y1 + initialDetection.Y2) / 2

	var maxUpwardDelta, maxDownwardDelta float32
	maxUpwardDelta = float32(-initialY) // Initialize with baseline

	ticker := time.NewTicker(50 * time.Millisecond)
	defer ticker.Stop()

	timeout := time.After(time.Duration(cs.config.TimeoutSeconds) * time.Second)

	for {
		select {
		case <-ctx.Done():
			return false
		case <-timeout:
			// Check if we detected a nod pattern
			return maxUpwardDelta > 10 && maxDownwardDelta > 10
		case <-ticker.C:
			// Get frame and detect face
			// In production, track face position continuously
			// For PoC, simulate detection

			// Simulate movement detection
			// This would use actual face detection in production
		}
	}
}

// detectTurn detects head turning
func (cs *ChallengeSystem) detectTurn(
	ctx context.Context,
	cam *camera.Camera,
	initialDetection models.Detection,
	direction ChallengeType,
) bool {
	// Turn detection using horizontal face position change
	// or using pose estimation (yaw angle)

	initialX := (initialDetection.X1 + initialDetection.X2) / 2
	initialWidth := initialDetection.X2 - initialDetection.X1

	ticker := time.NewTicker(50 * time.Millisecond)
	defer ticker.Stop()

	timeout := time.After(time.Duration(cs.config.TimeoutSeconds) * time.Second)

	for {
		select {
		case <-ctx.Done():
			return false
		case <-timeout:
			return false
		case <-ticker.C:
			// Get frame
			frame, ok := cam.GetFrame()
			if !ok {
				continue
			}

			img, err := frame.ToImage()
			if err != nil {
				continue
			}

			// In production, use head pose estimation
			// For PoC, use face position shift as proxy

			// Detect face (simplified)
			_ = img

			// Check if face has moved sufficiently
			// In production, use actual pose estimation
			currentX := initialX // Would be actual detection
			deltaX := currentX - initialX

			// Normalize by face width
			normalizedDelta := deltaX / initialWidth

			if direction == ChallengeTurnLeft && normalizedDelta < -0.3 {
				return true
			}
			if direction == ChallengeTurnRight && normalizedDelta > 0.3 {
				return true
			}
		}
	}
}

// HeadPose represents head pose estimation
type HeadPose struct {
	Yaw   float64 // Left/right rotation
	Pitch float64 // Up/down rotation
	Roll  float64 // Tilt rotation
}

// EstimateHeadPose estimates head pose from face landmarks
// This is a simplified implementation
func EstimateHeadPose(landmarks [][2]float32) HeadPose {
	if len(landmarks) < 5 {
		return HeadPose{}
	}

	// Use eye positions and nose for pose estimation
	leftEye := landmarks[0]
	rightEye := landmarks[1]
	nose := landmarks[2]

	// Calculate yaw from eye-nose triangle
	// Simplified - in production use proper 3D pose estimation
	eyeCenterX := (leftEye[0] + rightEye[0]) / 2
	noseOffset := nose[0] - eyeCenterX

	yaw := float64(noseOffset) * 2.0 // Rough approximation

	// Calculate pitch from eye-nose vertical
	eyeCenterY := (leftEye[1] + rightEye[1]) / 2
	verticalOffset := nose[1] - eyeCenterY

	pitch := float64(verticalOffset) * 1.5

	// Calculate roll from eye line
	eyeDeltaY := rightEye[1] - leftEye[1]
	eyeDeltaX := rightEye[0] - leftEye[0]

	roll := math.Atan2(float64(eyeDeltaY), float64(eyeDeltaX)) * 180 / math.Pi

	return HeadPose{
		Yaw:   yaw,
		Pitch: pitch,
		Roll:  roll,
	}
}

// EyeAspectRatio calculates the eye aspect ratio for blink detection
func EyeAspectRatio(eyeLandmarks [][2]float32) float64 {
	if len(eyeLandmarks) < 6 {
		return 1.0 // Open by default
	}

	// For a typical 6-point eye landmark:
	// P0, P3 are horizontal corners
	// P1, P2, P4, P5 are vertical points

	// Vertical distances
	A := distance(eyeLandmarks[1], eyeLandmarks[5])
	B := distance(eyeLandmarks[2], eyeLandmarks[4])

	// Horizontal distance
	C := distance(eyeLandmarks[0], eyeLandmarks[3])

	if C == 0 {
		return 1.0
	}

	// EAR = (A + B) / (2 * C)
	return (A + B) / (2 * C)
}

// distance calculates Euclidean distance between two points
func distance(p1, p2 [2]float32) float64 {
	dx := float64(p1[0] - p2[0])
	dy := float64(p1[1] - p2[1])
	return math.Sqrt(dx*dx + dy*dy)
}
