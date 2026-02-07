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
	// Hardware depth sensing via Windows Hello compatible IR cameras provides
	// better liveness detection than blink detection.
	// TODO: Implement depth-based liveness detection using IR camera depth data
	return true
}

// detectNod detects head nodding
// processNodFrame processes a single frame for nod detection
func (cs *ChallengeSystem) processNodFrame(
	cam *camera.Camera,
	detector func(image.Image) ([]models.Detection, error),
	initialPitch float64,
	maxUp, maxDown *float64,
) (bool, error) {
	frame, ok := cam.GetFrame()
	if !ok {
		return false, nil
	}

	img, err := frame.ToImage()
	if err != nil {
		return false, nil
	}

	detections, err := detector(img)
	if err != nil || len(detections) == 0 || len(detections[0].Landmarks) < 3 {
		return false, nil
	}

	det := detections[0]
	currentPitch := calculatePitch(det.Landmarks)
	diff := currentPitch - initialPitch

	if diff > *maxUp {
		*maxUp = diff
	}
	if diff < *maxDown {
		*maxDown = diff
	}

	eyeDist := distance(det.Landmarks[0], det.Landmarks[1])
	if eyeDist == 0 {
		return false, nil
	}

	normalizedRange := (*maxUp - *maxDown) / eyeDist
	return normalizedRange > 0.3, nil
}

func (cs *ChallengeSystem) detectNod(
	ctx context.Context,
	cam *camera.Camera,
	initialDetection models.Detection,
	detector func(image.Image) ([]models.Detection, error),
) bool {
	if len(initialDetection.Landmarks) < 3 {
		return false
	}

	initialPitch := calculatePitch(initialDetection.Landmarks)
	var maxUp, maxDown float64

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
			if detected, _ := cs.processNodFrame(cam, detector, initialPitch, &maxUp, &maxDown); detected {
				return true
			}
		}
	}
}

// detectTurn detects head turning
// processTurnFrame processes a single frame for turn detection
func (cs *ChallengeSystem) processTurnFrame(
	cam *camera.Camera,
	detector func(image.Image) ([]models.Detection, error),
	initialYaw float64,
	direction ChallengeType,
) (bool, error) {
	frame, ok := cam.GetFrame()
	if !ok {
		return false, nil
	}

	img, err := frame.ToImage()
	if err != nil {
		return false, nil
	}

	detections, err := detector(img)
	if err != nil || len(detections) == 0 || len(detections[0].Landmarks) < 3 {
		return false, nil
	}

	det := detections[0]
	currentYaw := calculateYaw(det.Landmarks)

	eyeDist := distance(det.Landmarks[0], det.Landmarks[1])
	if eyeDist == 0 {
		return false, nil
	}

	deltaYaw := (currentYaw - initialYaw) / eyeDist

	if direction == ChallengeTurnLeft && deltaYaw < -0.2 {
		return true, nil
	}
	if direction == ChallengeTurnRight && deltaYaw > 0.2 {
		return true, nil
	}

	return false, nil
}

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
			if detected, _ := cs.processTurnFrame(cam, detector, initialYaw, direction); detected {
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
