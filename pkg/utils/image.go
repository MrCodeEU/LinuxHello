// Package utils provides utility functions for image processing
package utils

import (
	"image"
	"image/color"
	"math"
)

// ResizeImage resizes an image using bilinear interpolation
func ResizeImage(src image.Image, dstWidth, dstHeight int) image.Image {
	srcBounds := src.Bounds()
	srcWidth := srcBounds.Dx()
	srcHeight := srcBounds.Dy()

	dst := image.NewRGBA(image.Rect(0, 0, dstWidth, dstHeight))

	for y := 0; y < dstHeight; y++ {
		for x := 0; x < dstWidth; x++ {
			// Map to source coordinates
			srcX := float64(x) * float64(srcWidth) / float64(dstWidth)
			srcY := float64(y) * float64(srcHeight) / float64(dstHeight)

			// Sample pixel
			r, g, b := SamplePixelBilinear(src, srcX, srcY)

			dst.Set(x, y, color.RGBA{R: uint8(r), G: uint8(g), B: uint8(b), A: 255})
		}
	}

	return dst
}

// SamplePixelBilinear samples a pixel using bilinear interpolation
func SamplePixelBilinear(img image.Image, x, y float64) (float64, float64, float64) {
	bounds := img.Bounds()
	x0 := int(math.Floor(x))
	y0 := int(math.Floor(y))
	x1 := x0 + 1
	y1 := y0 + 1

	// Clamp to bounds
	if x0 < bounds.Min.X {
		x0 = bounds.Min.X
	}
	if y0 < bounds.Min.Y {
		y0 = bounds.Min.Y
	}
	if x1 >= bounds.Max.X {
		x1 = bounds.Max.X - 1
	}
	if y1 >= bounds.Max.Y {
		y1 = bounds.Max.Y - 1
	}

	// Bilinear interpolation weights
	fx := x - float64(x0)
	fy := y - float64(y0)

	// Sample four corners
	r00, g00, b00, _ := img.At(x0, y0).RGBA()
	r01, g01, b01, _ := img.At(x0, y1).RGBA()
	r10, g10, b10, _ := img.At(x1, y0).RGBA()
	r11, g11, b11, _ := img.At(x1, y1).RGBA()

	// Convert from 16-bit to 8-bit
	r00, g00, b00 = r00>>8, g00>>8, b00>>8
	r01, g01, b01 = r01>>8, g01>>8, b01>>8
	r10, g10, b10 = r10>>8, g10>>8, b10>>8
	r11, g11, b11 = r11>>8, g11>>8, b11>>8

	// Interpolate
	r := (1-fx)*(1-fy)*float64(r00) + (1-fx)*fy*float64(r01) +
		fx*(1-fy)*float64(r10) + fx*fy*float64(r11)
	g := (1-fx)*(1-fy)*float64(g00) + (1-fx)*fy*float64(g01) +
		fx*(1-fy)*float64(g10) + fx*fy*float64(g11)
	b := (1-fx)*(1-fy)*float64(b00) + (1-fx)*fy*float64(b01) +
		fx*(1-fy)*float64(b10) + fx*fy*float64(b11)

	return r, g, b
}

// ImageToFloat32 converts an image to a float32 array in CHW format
// normalized to [-1, 1] for model input
func ImageToFloat32(img image.Image, targetSize int) []float32 {
	// Resize image
	resized := ResizeImage(img, targetSize, targetSize)

	// Convert to float32 array [3, H, W]
	data := make([]float32, 3*targetSize*targetSize)

	for y := 0; y < targetSize; y++ {
		for x := 0; x < targetSize; x++ {
			r, g, b, _ := resized.At(x, y).RGBA()

			// Convert from 16-bit to float and normalize to [-1, 1]
			idx := y*targetSize + x
			data[idx] = (float32(r>>8)/255.0 - 0.5) * 2.0                         // R
			data[idx+targetSize*targetSize] = (float32(g>>8)/255.0 - 0.5) * 2.0   // G
			data[idx+2*targetSize*targetSize] = (float32(b>>8)/255.0 - 0.5) * 2.0 // B
		}
	}

	return data
}

// ImageToFloat32Normalized converts an image to a float32 array in CHW format
// normalized to [0, 1]
func ImageToFloat32Normalized(img image.Image, targetSize int) []float32 {
	// Resize image
	resized := ResizeImage(img, targetSize, targetSize)

	// Convert to float32 array [3, H, W]
	data := make([]float32, 3*targetSize*targetSize)

	for y := 0; y < targetSize; y++ {
		for x := 0; x < targetSize; x++ {
			r, g, b, _ := resized.At(x, y).RGBA()

			// Convert from 16-bit to float and normalize to [0, 1]
			idx := y*targetSize + x
			data[idx] = float32(r>>8) / 255.0                         // R
			data[idx+targetSize*targetSize] = float32(g>>8) / 255.0   // G
			data[idx+2*targetSize*targetSize] = float32(b>>8) / 255.0 // B
		}
	}

	return data
}

// CropImage crops a region from an image
func CropImage(img image.Image, x, y, width, height int) image.Image {
	bounds := img.Bounds()

	// Clamp to bounds
	if x < bounds.Min.X {
		x = bounds.Min.X
	}
	if y < bounds.Min.Y {
		y = bounds.Min.Y
	}
	if x+width > bounds.Max.X {
		width = bounds.Max.X - x
	}
	if y+height > bounds.Max.Y {
		height = bounds.Max.Y - y
	}

	cropped := image.NewRGBA(image.Rect(0, 0, width, height))

	for j := 0; j < height; j++ {
		for i := 0; i < width; i++ {
			r, g, b, a := img.At(x+i, y+j).RGBA()
			cropped.Set(i, j, color.RGBA{R: uint8(r >> 8), G: uint8(g >> 8), B: uint8(b >> 8), A: uint8(a >> 8)})
		}
	}

	return cropped
}

// Grayscale converts an image to grayscale
func Grayscale(img image.Image) image.Image {
	bounds := img.Bounds()
	gray := image.NewGray(bounds)

	for y := bounds.Min.Y; y < bounds.Max.Y; y++ {
		for x := bounds.Min.X; x < bounds.Max.X; x++ {
			r, g, b, _ := img.At(x, y).RGBA()
			// Convert to grayscale using luminance formula
			luma := 0.299*float64(r) + 0.587*float64(g) + 0.114*float64(b)
			gray.Set(x, y, color.Gray{Y: uint8(luma / 256)})
		}
	}

	return gray
}

// Clamp clamps a value between min and max
func Clamp(val, min, max float64) float64 {
	if val < min {
		return min
	}
	if val > max {
		return max
	}
	return val
}
