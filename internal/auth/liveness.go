// Package auth provides liveness detection functionality
package auth

import (
	"image"
	"math"
)

// LivenessDetector provides basic liveness detection using depth variance
type LivenessDetector struct {
	depthThreshold    float64
	varianceThreshold float64
}

// NewLivenessDetector creates a new liveness detector
func NewLivenessDetector(depthThreshold, varianceThreshold float64) *LivenessDetector {
	return &LivenessDetector{
		depthThreshold:    depthThreshold,
		varianceThreshold: varianceThreshold,
	}
}

// CheckLiveness performs basic liveness detection on an image
// Returns (isLive, confidence, error)
func (ld *LivenessDetector) CheckLiveness(img image.Image) (bool, float64, error) {
	// Calculate grayscale variance as a simple liveness indicator
	// Real photos/screens tend to have lower variance than live faces
	variance := calculateImageVariance(img)

	// Calculate edge density (live faces have more natural edges)
	edgeDensity := calculateEdgeDensity(img)

	// Calculate texture complexity
	textureScore := calculateTextureComplexity(img)

	// Combine metrics for confidence score
	// Higher variance, edge density, and texture indicate live face
	confidence := (normalizeScore(variance, 0, 10000) * 0.4) +
		(edgeDensity * 0.3) +
		(textureScore * 0.3)

	// Determine if live based on combined score
	isLive := confidence > 0.5 && variance > ld.varianceThreshold

	return isLive, confidence, nil
}

// calculateImageVariance calculates the variance of pixel intensities
func calculateImageVariance(img image.Image) float64 {
	bounds := img.Bounds()

	var sum, sumSq float64
	count := 0

	for y := bounds.Min.Y; y < bounds.Max.Y; y++ {
		for x := bounds.Min.X; x < bounds.Max.X; x++ {
			r, g, b, _ := img.At(x, y).RGBA()
			// Convert to grayscale (0-255)
			gray := (0.299*float64(r) + 0.587*float64(g) + 0.114*float64(b)) / 256.0

			sum += gray
			sumSq += gray * gray
			count++
		}
	}

	if count == 0 {
		return 0
	}

	mean := sum / float64(count)
	variance := (sumSq / float64(count)) - (mean * mean)

	return variance
}

// calculateEdgeDensity calculates the density of edges in the image
func calculateEdgeDensity(img image.Image) float64 {
	bounds := img.Bounds()

	if bounds.Dx() < 2 || bounds.Dy() < 2 {
		return 0
	}

	edgeCount := 0
	totalPixels := 0

	// Simple Sobel edge detection
	for y := bounds.Min.Y + 1; y < bounds.Max.Y-1; y++ {
		for x := bounds.Min.X + 1; x < bounds.Max.X-1; x++ {
			gx := getGrayValue(img, x+1, y) - getGrayValue(img, x-1, y)
			gy := getGrayValue(img, x, y+1) - getGrayValue(img, x, y-1)

			gradient := math.Sqrt(float64(gx*gx + gy*gy))

			if gradient > 30 { // Edge threshold
				edgeCount++
			}
			totalPixels++
		}
	}

	if totalPixels == 0 {
		return 0
	}

	return float64(edgeCount) / float64(totalPixels)
}

// calculateTextureComplexity calculates local binary pattern variance
func calculateTextureComplexity(img image.Image) float64 {
	bounds := img.Bounds()
	width, height := bounds.Dx(), bounds.Dy()

	if width < 3 || height < 3 {
		return 0
	}

	var lbpVariance float64
	sampleCount := 0

	// Sample LBP at regular intervals
	step := 8
	for y := bounds.Min.Y + 1; y < bounds.Max.Y-1; y += step {
		for x := bounds.Min.X + 1; x < bounds.Max.X-1; x += step {
			center := getGrayValue(img, x, y)

			// 8-neighbor LBP
			var pattern uint8
			if getGrayValue(img, x-1, y-1) >= center {
				pattern |= 1 << 0
			}
			if getGrayValue(img, x, y-1) >= center {
				pattern |= 1 << 1
			}
			if getGrayValue(img, x+1, y-1) >= center {
				pattern |= 1 << 2
			}
			if getGrayValue(img, x+1, y) >= center {
				pattern |= 1 << 3
			}
			if getGrayValue(img, x+1, y+1) >= center {
				pattern |= 1 << 4
			}
			if getGrayValue(img, x, y+1) >= center {
				pattern |= 1 << 5
			}
			if getGrayValue(img, x-1, y+1) >= center {
				pattern |= 1 << 6
			}
			if getGrayValue(img, x-1, y) >= center {
				pattern |= 1 << 7
			}

			lbpVariance += float64(pattern)
			sampleCount++
		}
	}

	if sampleCount == 0 {
		return 0
	}

	// Normalize to 0-1 range
	avgPattern := lbpVariance / float64(sampleCount)
	return normalizeScore(avgPattern, 0, 255)
}

// getGrayValue returns grayscale value (0-255) for a pixel
func getGrayValue(img image.Image, x, y int) int {
	r, g, b, _ := img.At(x, y).RGBA()
	return int((0.299*float64(r) + 0.587*float64(g) + 0.114*float64(b)) / 256.0)
}

// normalizeScore normalizes a score to 0-1 range
func normalizeScore(value, min, max float64) float64 {
	if max <= min {
		return 0
	}
	normalized := (value - min) / (max - min)
	if normalized < 0 {
		return 0
	}
	if normalized > 1 {
		return 1
	}
	return normalized
}
