// Package camera provides video capture functionality using V4L2
package camera

import (
	"bytes"
	"context"
	"fmt"
	"image"
	"image/color"
	"image/jpeg"
	"os/exec"
	"sync"
	"time"

	"github.com/MrCodeEU/LinuxHello/internal/config"
	"github.com/vladimirvivien/go4vl/device"
	"github.com/vladimirvivien/go4vl/v4l2"
)

// Frame represents a captured video frame
type Frame struct {
	Data      []byte
	Width     int
	Height    int
	Format    v4l2.FourCCType
	Timestamp time.Time
	Sequence  uint32
}

// ToImage converts the frame to a Go image.Image
func (f *Frame) ToImage() (image.Image, error) {
	switch f.Format {
	case v4l2.PixelFmtMJPEG:
		return jpeg.Decode(bytes.NewReader(f.Data))
	case v4l2.PixelFmtYUYV:
		// Convert YUYV to RGB
		return yuyvToRGB(f.Data, f.Width, f.Height)
	case v4l2.PixelFmtRGB24:
		return rgb24ToImage(f.Data, f.Width, f.Height)
	case v4l2.PixelFmtGrey:
		return greyToImage(f.Data, f.Width, f.Height)
	default:
		return nil, fmt.Errorf("unsupported pixel format: %v", f.Format)
	}
}

// Camera represents a V4L2 camera device
type Camera struct {
	device     *device.Device
	config     config.CameraConfig
	frameChan  chan *Frame
	ctx        context.Context
	cancel     context.CancelFunc
	mu         sync.RWMutex // Protect concurrent access
	isStopping bool         // Flag to prevent concurrent stops
	isRunning  bool
	logger     Logger
	wg         sync.WaitGroup
}

// Logger is a simple interface for logging
type Logger interface {
	Infof(format string, args ...interface{})
}

type defaultLogger struct{}

func (l *defaultLogger) Infof(format string, args ...interface{}) {
	// No-op by default
}

// NewCamera creates a new camera instance
func NewCamera(cfg config.CameraConfig) (*Camera, error) {
	// Open the device
	dev, err := device.Open(cfg.Device)
	if err != nil {
		return nil, fmt.Errorf("failed to open camera device %s: %w", cfg.Device, err)
	}

	return &Camera{
		device:    dev,
		config:    cfg,
		frameChan: make(chan *Frame, 4),
		logger:    &defaultLogger{},
	}, nil
}

// Initialize configures the camera with the specified settings
func (c *Camera) Initialize() error {
	c.mu.Lock()
	defer c.mu.Unlock()

	// Camera initialization with go4vl is simplified
	// The device handles format negotiation automatically
	c.logger.Infof("Camera %s initialized successfully", c.config.Device)
	return nil
}

func triggerIREmitter(_ string) error {
	// Check if linux-enable-ir-emitter exists
	_, err := exec.LookPath("linux-enable-ir-emitter")
	if err != nil {
		return fmt.Errorf("linux-enable-ir-emitter tool not found")
	}

	// Run the command: linux-enable-ir-emitter run
	cmd := exec.Command("linux-enable-ir-emitter", "run")
	if output, err := cmd.CombinedOutput(); err != nil {
		return fmt.Errorf("failed to enable IR emitter: %w (output: %s)", err, output)
	}

	return nil
}

// TriggerIR attempts to trigger the IR emitter
func (c *Camera) TriggerIR() error {
	return triggerIREmitter(c.config.Device)
}

// Start begins video capture
func (c *Camera) Start() error {
	c.mu.Lock()
	defer c.mu.Unlock()

	if c.isRunning {
		return nil
	}

	// If device was closed, reopen it
	if c.device == nil {
		dev, err := device.Open(c.config.Device)
		if err != nil {
			return fmt.Errorf("failed to reopen camera device %s: %w", c.config.Device, err)
		}
		c.device = dev
	}

	c.ctx, c.cancel = context.WithCancel(context.Background())

	// Start the device
	if err := c.device.Start(c.ctx); err != nil {
		return fmt.Errorf("failed to start camera: %w", err)
	}

	// Get actual resolution after negotiation
	if fmtDesc, err := c.device.GetPixFormat(); err == nil {
		actualWidth := int(fmtDesc.Width)
		actualHeight := int(fmtDesc.Height)

		if actualWidth != c.config.Width || actualHeight != c.config.Height {
			c.logger.Infof("Camera resolution mismatch! Config: %dx%d, Actual: %dx%d - Using actual dimensions",
				c.config.Width, c.config.Height, actualWidth, actualHeight)
			// Update config to match actual
			c.config.Width = actualWidth
			c.config.Height = actualHeight
		} else {
			c.logger.Infof("Camera resolution confirmed: %dx%d", actualWidth, actualHeight)
		}
	} else {
		c.logger.Infof("Warning: Could not verify camera resolution: %v", err)
	}

	c.isRunning = true

	// Trigger IR emitter after starting the stream
	if err := c.TriggerIR(); err != nil {
		c.logger.Infof("Note: IR emitter trigger skipped or failed: %v", err)
	}

	// Start frame capture goroutine
	c.wg.Add(1)
	go c.captureLoop()

	return nil
}

// Stop stops video capture
func (c *Camera) Stop() error {
	c.mu.Lock()
	defer c.mu.Unlock()

	if !c.isRunning || c.isStopping {
		return nil
	}

	c.isStopping = true
	c.performSafeShutdown()
	c.logger.Infof("Camera stopped successfully")
	return nil
}

// performSafeShutdown safely shuts down the camera with panic recovery
func (c *Camera) performSafeShutdown() {
	// Use defer with recover to handle any panics from go4vl cleanup
	defer func() {
		if r := recover(); r != nil {
			c.logger.Infof("Recovered from panic during camera stop: %v", r)
		}
	}()

	// 1. Signal cancellation
	c.cancelCapture()

	// 2. Wait for our local capture loop to exit
	c.wg.Wait()

	// 3. Drain the go4vl channel to ensure the external goroutine finishes
	// The go4vl library should close the channel when context is done.
	// We drain it to prevent 'send on closed channel' panics if we were to close the device too early.
	done := make(chan struct{})
	go func() {
		out := c.device.GetOutput()
		for range out {
			// Drain
		}
		close(done)
	}()

	select {
	case <-done:
		// Channel closed naturally
	case <-time.After(500 * time.Millisecond):
		c.logger.Infof("Timed out waiting for camera channel to close")
	}

	// 4. Now it's safe(r) to stop and close the device
	c.stopDevice()
	c.closeDevice()
	c.resetState()
}

// cancelCapture cancels the capture context
func (c *Camera) cancelCapture() {
	if c.cancel != nil {
		c.cancel()
	}
}

// stopDevice safely stops the camera device
func (c *Camera) stopDevice() {
	if c.device == nil {
		return
	}

	func() {
		defer func() {
			if r := recover(); r != nil {
				c.logger.Infof("Recovered from device stop panic: %v", r)
			}
		}()
		_ = c.device.Stop()
	}()
}

// closeDevice safely closes the camera device
func (c *Camera) closeDevice() {
	if c.device == nil {
		return
	}

	func() {
		defer func() {
			if r := recover(); r != nil {
				c.logger.Infof("Recovered from device close panic: %v", r)
			}
		}()
		_ = c.device.Close()
	}()

	c.device = nil
}

// resetState resets the camera state for potential restart
func (c *Camera) resetState() {
	c.isRunning = false
	c.isStopping = false
}

// GetFrame returns the next available frame (thread-safe)
func (c *Camera) GetFrame() (*Frame, bool) {
	c.mu.RLock()
	defer c.mu.RUnlock()

	select {
	case frame, ok := <-c.frameChan:
		return frame, ok
	case <-time.After(5 * time.Second):
		return nil, false
	}
}

// GetFrameChan returns the frame channel for streaming (thread-safe)
func (c *Camera) GetFrameChan() <-chan *Frame {
	c.mu.RLock()
	defer c.mu.RUnlock()
	return c.frameChan
}

// captureLoop continuously captures frames from the camera
func (c *Camera) captureLoop() {
	defer c.wg.Done()
	frameChan := c.device.GetOutput()

	firstFrame := true

	for {
		select {
		case <-c.ctx.Done():
			return
		case buf, ok := <-frameChan:
			if !ok {
				return
			}

			// Make a copy of the buffer data
			dataCopy := make([]byte, len(buf))
			copy(dataCopy, buf)

			// Determine pixel format based on config
			var pixelFormat v4l2.FourCCType
			switch c.config.PixelFormat {
			case "GREY":
				pixelFormat = v4l2.PixelFmtGrey
			case "YUYV":
				pixelFormat = v4l2.PixelFmtYUYV
			case "RGB24":
				pixelFormat = v4l2.PixelFmtRGB24
			case "MJPEG", "":
				pixelFormat = v4l2.PixelFmtMJPEG
			default:
				pixelFormat = v4l2.PixelFmtGrey
			}

			frame := &Frame{
				Data:      dataCopy,
				Width:     c.config.Width,
				Height:    c.config.Height,
				Format:    pixelFormat,
				Timestamp: time.Now(),
			}

			if firstFrame {
				firstFrame = false
			}

			// Non-blocking send
			select {
			case c.frameChan <- frame:
			case <-c.ctx.Done():
				return
			default:
				// Busy, drop frame
			}
		}
	}
}

// Close releases camera resources
func (c *Camera) Close() error {
	_ = c.Stop()

	if c.device != nil {
		return c.device.Close()
	}
	return nil
}

// GetSupportedFormats returns the list of supported pixel formats
func (c *Camera) GetSupportedFormats() ([]v4l2.FormatDescription, error) {
	return c.device.GetFormatDescriptions()
}

// GetDeviceInfo returns information about the camera device
func (c *Camera) GetDeviceInfo() (string, error) {
	return c.config.Device, nil
}

// IRCamera represents an infrared camera device
type IRCamera struct {
	*Camera
}

// NewIRCamera creates a new IR camera instance
func NewIRCamera(cfg config.CameraConfig) (*IRCamera, error) {
	// Override pixel format for IR
	cfg.PixelFormat = "Y16" // 16-bit grayscale for IR

	cam, err := NewCamera(cfg)
	if err != nil {
		return nil, err
	}

	return &IRCamera{Camera: cam}, nil
}

// Helper functions for format conversion

func yuyvToRGB(data []byte, width, height int) (image.Image, error) {
	img := image.NewRGBA(image.Rect(0, 0, width, height))

	for y := 0; y < height; y++ {
		for x := 0; x < width; x += 2 {
			idx := (y*width + x) * 2
			if idx+3 >= len(data) {
				break
			}

			Y0 := int(data[idx])
			U := int(data[idx+1]) - 128
			Y1 := int(data[idx+2])
			V := int(data[idx+3]) - 128

			r0, g0, b0 := yuvToRGB(Y0, U, V)
			r1, g1, b1 := yuvToRGB(Y1, U, V)

			img.Set(x, y, color.RGBA{R: r0, G: g0, B: b0, A: 255})
			if x+1 < width {
				img.Set(x+1, y, color.RGBA{R: r1, G: g1, B: b1, A: 255})
			}
		}
	}

	return img, nil
}

func yuvToRGB(y, u, v int) (uint8, uint8, uint8) {
	c := y - 16
	d := u
	e := v

	R := (298*c + 409*e + 128) >> 8
	G := (298*c - 100*d - 208*e + 128) >> 8
	B := (298*c + 516*d + 128) >> 8

	return clampUint8(R), clampUint8(G), clampUint8(B)
}

func clampUint8(val int) uint8 {
	if val < 0 {
		return 0
	}
	if val > 255 {
		return 255
	}
	return uint8(val)
}

func rgb24ToImage(data []byte, width, height int) (image.Image, error) {
	img := image.NewRGBA(image.Rect(0, 0, width, height))

	for y := 0; y < height; y++ {
		for x := 0; x < width; x++ {
			idx := (y*width + x) * 3
			if idx+2 >= len(data) {
				break
			}

			r := data[idx]
			g := data[idx+1]
			b := data[idx+2]

			img.Set(x, y, color.RGBA{R: r, G: g, B: b, A: 255})
		}
	}

	return img, nil
}

func greyToImage(data []byte, width, height int) (image.Image, error) {
	img := image.NewGray(image.Rect(0, 0, width, height))
	copy(img.Pix, data)
	return img, nil
}
