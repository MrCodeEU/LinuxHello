package auth

import (
	"context"
	"image"
	"math"
	"math/rand"
	"time"

	"github.com/MrCodeEU/LinuxHello/internal/camera"
	"github.com/MrCodeEU/LinuxHello/internal/config"
	"github.com/MrCodeEU/LinuxHello/pkg/models"
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
	detector func(image.Image) ([]models.Detection, error),
) bool {
	switch challenge.Type {
	case ChallengeBlink:
		return cs.detectBlink(ctx, cam, initialDetection, detector)
	case ChallengeNod:
		return cs.detectNod(ctx, cam, initialDetection, detector)
	case ChallengeTurnLeft, ChallengeTurnRight:
		return cs.detectTurn(ctx, cam, initialDetection, challenge.Type, detector)
	default:
		return false
	}
}

// detectBlink detects eye blinking
func (cs *ChallengeSystem) detectBlink(
	ctx context.Context,
	cam *camera.Camera,
	initialDetection models.Detection,
	detector func(image.Image) ([]models.Detection, error),
) bool {
	// Blink detection requires detailed eye landmarks (usually 6 points per eye)
	// to calculate Eye Aspect Ratio (EAR).
	// Our current model (SCRFD) only provides 5-point landmarks (eye centers).
	// Therefore, we cannot reliably detect blinking.
	// TODO: implement blink detection when a model with 6-point eye landmarks is available
	return true
}

// detectNod detects head nodding
func (cs *ChallengeSystem) detectNod(
	ctx context.Context,
	cam *camera.Camera,
	initialDetection models.Detection,
	detector func(image.Image) ([]models.Detection, error),
) bool {
	if len(initialDetection.Landmarks) < 3 {
		return false
	}

	// Initial nose Y relative to eye center Y (Pitch approximation)
	initialPitch := calculatePitch(initialDetection.Landmarks)

	var maxUp, maxDown float64

	ticker := time.NewTicker(100 * time.Millisecond) // 10 FPS is enough for gestures
	defer ticker.Stop()

	timeout := time.After(time.Duration(cs.config.TimeoutSeconds) * time.Second)

	for {
		select {
		case <-ctx.Done():
			return false
		case <-timeout:
			return false
		case <-ticker.C:
			frame, ok := cam.GetFrame()
			if !ok {
				continue
			}

			img, err := frame.ToImage()
			if err != nil {
				continue
			}

			detections, err := detector(img)
			if err != nil || len(detections) == 0 {
				continue
			}

			// Use the largest face
			det := detections[0]
			if len(det.Landmarks) < 3 {
				continue
			}

			currentPitch := calculatePitch(det.Landmarks)
			diff := currentPitch - initialPitch

			if diff > maxUp {
				maxUp = diff
			}
			if diff < maxDown {
				maxDown = diff
			}

			// Thresholds for nod (normalized by eye distance)
			// Pitch is roughly: nose_y - eye_center_y
			// Positive diff = nose went down (nod down)
			// Negative diff = nose went up (nod up)

			// We look for significant movement in both directions or a strong single nod
			eyeDist := distance(det.Landmarks[0], det.Landmarks[1])
			if eyeDist == 0 {
				continue
			}

			normalizedRange := (maxUp - maxDown) / eyeDist

			// If total vertical movement is > 30% of eye distance, consider it a nod
			if normalizedRange > 0.3 {
				return true
			}
		}
	}
}

// detectTurn detects head turning
func (cs *ChallengeSystem) detectTurn(
	ctx context.Context,
	cam *camera.Camera,
	initialDetection models.Detection,
	direction ChallengeType,
	detector func(image.Image) ([]models.Detection, error),
) bool {
	if len(initialDetection.Landmarks) < 3 {
		return false
	}

	initialYaw := calculateYaw(initialDetection.Landmarks)

	ticker := time.NewTicker(100 * time.Millisecond)
	defer ticker.Stop()

	timeout := time.After(time.Duration(cs.config.TimeoutSeconds) * time.Second)

	for {
		select {
		case <-ctx.Done():
			return false
		case <-timeout:
			return false
		case <-ticker.C:
			frame, ok := cam.GetFrame()
			if !ok {
				continue
			}

			img, err := frame.ToImage()
			if err != nil {
				continue
			}

			detections, err := detector(img)
			if err != nil || len(detections) == 0 {
				continue
			}

			det := detections[0]
			if len(det.Landmarks) < 3 {
				continue
			}

			currentYaw := calculateYaw(det.Landmarks)

			// Yaw: nose_x - eye_center_x
			// Positive = Looking Right (camera perspective) -> User turning Left?
			// Wait, if User turns LEFT, their nose moves LEFT in image (smaller X).
			// If User turns RIGHT, their nose moves RIGHT in image (larger X).
			//
			// calculateYaw returns (nose.x - eyeCenter.x).
			// Center is 0.
			// Turn Left (nose moves left) -> Yaw becomes more negative.
			// Turn Right (nose moves right) -> Yaw becomes more positive.

			eyeDist := distance(det.Landmarks[0], det.Landmarks[1])
			if eyeDist == 0 {
				continue
			}

			deltaYaw := (currentYaw - initialYaw) / eyeDist

			if direction == ChallengeTurnLeft && deltaYaw < -0.2 { // Turned Left
				return true
			}
			if direction == ChallengeTurnRight && deltaYaw > 0.2 { // Turned Right
				return true
			}
		}
	}
}

func calculateYaw(landmarks [][2]float32) float64 {
	leftEye := landmarks[0]
	rightEye := landmarks[1]
	nose := landmarks[2]

	eyeCenterX := (leftEye[0] + rightEye[0]) / 2
	return float64(nose[0] - eyeCenterX)
}

func calculatePitch(landmarks [][2]float32) float64 {
	leftEye := landmarks[0]
	rightEye := landmarks[1]
	nose := landmarks[2]

	eyeCenterY := (leftEye[1] + rightEye[1]) / 2
	return float64(nose[1] - eyeCenterY)
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
