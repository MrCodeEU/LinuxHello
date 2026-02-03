// Package config provides configuration management for FaceLock
package config

import (
	"fmt"
	"os"
	"path/filepath"

	"github.com/spf13/viper"
)

// Config holds all configuration for the application
type Config struct {
	// Inference service settings
	Inference InferenceConfig `mapstructure:"inference"`

	// Camera settings
	Camera CameraConfig `mapstructure:"camera"`

	// Detection settings
	Detection DetectionConfig `mapstructure:"detection"`

	// Recognition settings
	Recognition RecognitionConfig `mapstructure:"recognition"`

	// Liveness settings
	Liveness LivenessConfig `mapstructure:"liveness"`

	// Challenge-response settings
	Challenge ChallengeConfig `mapstructure:"challenge"`

	// Authentication settings
	Auth AuthConfig `mapstructure:"auth"`

	// Storage settings
	Storage StorageConfig `mapstructure:"storage"`

	// Logging settings
	Logging LoggingConfig `mapstructure:"logging"`
}

// InferenceConfig holds inference service configuration
type InferenceConfig struct {
	Address string `mapstructure:"address"` // gRPC service address (e.g., localhost:50051)
	Timeout int    `mapstructure:"timeout"` // Request timeout in seconds
}

// CameraConfig holds camera-related configuration
type CameraConfig struct {
	Device       string `mapstructure:"device"`        // V4L2 device path (e.g., /dev/video0)
	IRDevice     string `mapstructure:"ir_device"`     // IR camera device path
	DepthDevice  string `mapstructure:"depth_device"`  // Depth camera device path
	Width        int    `mapstructure:"width"`         // Capture width
	Height       int    `mapstructure:"height"`        // Capture height
	FPS          int    `mapstructure:"fps"`           // Frames per second
	PixelFormat  string `mapstructure:"pixel_format"`  // V4L2 pixel format
	UseRealSense bool   `mapstructure:"use_realsense"` // Use RealSense SDK
	AutoExposure bool   `mapstructure:"auto_exposure"` // Enable auto exposure
}

// DetectionConfig holds face detection configuration
type DetectionConfig struct {
	ModelPath     string  `mapstructure:"model_path"`     // Path to SCRFD model
	Confidence    float32 `mapstructure:"confidence"`     // Detection confidence threshold
	NMSThreshold  float32 `mapstructure:"nms_threshold"`  // Non-maximum suppression threshold
	InputSize     int     `mapstructure:"input_size"`     // Model input size (e.g., 640)
	MaxDetections int     `mapstructure:"max_detections"` // Maximum faces to detect
}

// RecognitionConfig holds face recognition configuration
type RecognitionConfig struct {
	ModelPath           string  `mapstructure:"model_path"`           // Path to ArcFace model
	InputSize           int     `mapstructure:"input_size"`           // Model input size (e.g., 112)
	EmbeddingSize       int     `mapstructure:"embedding_size"`       // Embedding vector size
	SimilarityThreshold float64 `mapstructure:"similarity_threshold"` // Cosine similarity threshold
	EnrollmentSamples   int     `mapstructure:"enrollment_samples"`   // Samples to collect during enrollment
}

// LivenessConfig holds liveness detection configuration
type LivenessConfig struct {
	Enabled             bool    `mapstructure:"enabled"`              // Enable liveness detection
	ModelPath           string  `mapstructure:"model_path"`           // Path to depth liveness model
	DepthThreshold      float32 `mapstructure:"depth_threshold"`      // Minimum depth variance
	ConfidenceThreshold float32 `mapstructure:"confidence_threshold"` // Liveness confidence threshold
	UseDepthCamera      bool    `mapstructure:"use_depth_camera"`     // Use depth camera data
	UseIRAnalysis       bool    `mapstructure:"use_ir_analysis"`      // Use IR reflection analysis
}

// ChallengeConfig holds challenge-response configuration
type ChallengeConfig struct {
	Enabled         bool     `mapstructure:"enabled"`          // Enable challenge-response
	ChallengeTypes  []string `mapstructure:"challenge_types"`  // Types: "blink", "nod", "turn_left", "turn_right"
	TimeoutSeconds  int      `mapstructure:"timeout_seconds"`  // Challenge timeout
	RequiredSuccess int      `mapstructure:"required_success"` // Required successful challenges
}

// AuthConfig holds authentication configuration
type AuthConfig struct {
	MaxAttempts     int    `mapstructure:"max_attempts"`     // Max auth attempts before lockout
	LockoutDuration int    `mapstructure:"lockout_duration"` // Lockout duration in seconds
	SessionTimeout  int    `mapstructure:"session_timeout"`  // Session timeout in seconds
	FallbackEnabled bool   `mapstructure:"fallback_enabled"` // Allow password fallback
	ContinuousAuth  bool   `mapstructure:"continuous_auth"`  // Continuous authentication mode
	SecurityLevel   string `mapstructure:"security_level"`   // "low", "medium", "high"
}

// StorageConfig holds data storage configuration
type StorageConfig struct {
	DataDir       string `mapstructure:"data_dir"`       // Directory for face data
	DatabasePath  string `mapstructure:"database_path"`  // SQLite database path
	MaxUsers      int    `mapstructure:"max_users"`      // Maximum enrolled users
	BackupEnabled bool   `mapstructure:"backup_enabled"` // Enable automatic backups
}

// LoggingConfig holds logging configuration
type LoggingConfig struct {
	Level      string `mapstructure:"level"`       // Log level: debug, info, warn, error
	File       string `mapstructure:"file"`        // Log file path (empty = stdout)
	MaxSize    int    `mapstructure:"max_size"`    // Max log file size in MB
	MaxBackups int    `mapstructure:"max_backups"` // Max number of backups
	MaxAge     int    `mapstructure:"max_age"`     // Max age in days
}

// DefaultConfig returns a configuration with default values
func DefaultConfig() *Config {
	return &Config{
		Inference: InferenceConfig{
			Address: "localhost:50051",
			Timeout: 10,
		},
		Camera: CameraConfig{
			Device:       "/dev/video0",
			IRDevice:     "",
			DepthDevice:  "",
			Width:        640,
			Height:       480,
			FPS:          30,
			PixelFormat:  "MJPEG",
			UseRealSense: false,
			AutoExposure: true,
		},
		Detection: DetectionConfig{
			ModelPath:     "models/scrfd_person_2.5g.onnx",
			Confidence:    0.5,
			NMSThreshold:  0.4,
			InputSize:     640,
			MaxDetections: 1,
		},
		Recognition: RecognitionConfig{
			ModelPath:           "models/arcface_r50.onnx",
			InputSize:           112,
			EmbeddingSize:       512,
			SimilarityThreshold: 0.6,
			EnrollmentSamples:   5,
		},
		Liveness: LivenessConfig{
			Enabled:             true,
			ModelPath:           "models/depth_liveness.onnx",
			DepthThreshold:      0.1,
			ConfidenceThreshold: 0.8,
			UseDepthCamera:      false,
			UseIRAnalysis:       true,
		},
		Challenge: ChallengeConfig{
			Enabled:         false,
			ChallengeTypes:  []string{"blink"},
			TimeoutSeconds:  10,
			RequiredSuccess: 1,
		},
		Auth: AuthConfig{
			MaxAttempts:     3,
			LockoutDuration: 300,
			SessionTimeout:  3600,
			FallbackEnabled: true,
			ContinuousAuth:  false,
			SecurityLevel:   "medium",
		},
		Storage: StorageConfig{
			DataDir:       "/var/lib/facelock",
			DatabasePath:  "/var/lib/facelock/facelock.db",
			MaxUsers:      100,
			BackupEnabled: true,
		},
		Logging: LoggingConfig{
			Level:      "info",
			File:       "/var/log/facelock.log",
			MaxSize:    100,
			MaxBackups: 3,
			MaxAge:     30,
		},
	}
}

// Load loads configuration from file and environment variables
func Load(configPath string) (*Config, error) {
	cfg := DefaultConfig()

	viper.SetConfigType("yaml")

	// Set config file if provided
	if configPath != "" {
		viper.SetConfigFile(configPath)
	} else {
		// Search for config in standard locations
		viper.SetConfigName("facelock")
		viper.AddConfigPath("/etc/facelock/")
		viper.AddConfigPath("$HOME/.facelock")
		viper.AddConfigPath(".")
	}

	// Environment variable prefix
	viper.SetEnvPrefix("FACELOCK")
	viper.AutomaticEnv()

	// Read config file (optional)
	if err := viper.ReadInConfig(); err != nil {
		// Config file not found is OK, use defaults
		if _, ok := err.(viper.ConfigFileNotFoundError); !ok {
			return nil, fmt.Errorf("error reading config: %w", err)
		}
	}

	// Unmarshal into struct
	if err := viper.Unmarshal(cfg); err != nil {
		return nil, fmt.Errorf("error unmarshaling config: %w", err)
	}

	// Ensure data directory exists
	if err := os.MkdirAll(cfg.Storage.DataDir, 0755); err != nil {
		return nil, fmt.Errorf("error creating data directory: %w", err)
	}

	return cfg, nil
}

// Save saves the configuration to a file
func (c *Config) Save(path string) error {
	viper.Reset()

	// Set values from struct
	viper.Set("camera", c.Camera)
	viper.Set("detection", c.Detection)
	viper.Set("recognition", c.Recognition)
	viper.Set("liveness", c.Liveness)
	viper.Set("challenge", c.Challenge)
	viper.Set("auth", c.Auth)
	viper.Set("storage", c.Storage)
	viper.Set("logging", c.Logging)

	// Ensure directory exists
	dir := filepath.Dir(path)
	if err := os.MkdirAll(dir, 0755); err != nil {
		return fmt.Errorf("error creating config directory: %w", err)
	}

	// Write config file
	if err := viper.WriteConfigAs(path); err != nil {
		return fmt.Errorf("error writing config: %w", err)
	}

	return nil
}

// Validate validates the configuration
func (c *Config) Validate() error {
	// Validate camera settings
	if c.Camera.Device == "" {
		return fmt.Errorf("camera device cannot be empty")
	}
	if c.Camera.Width <= 0 || c.Camera.Height <= 0 {
		return fmt.Errorf("invalid camera resolution: %dx%d", c.Camera.Width, c.Camera.Height)
	}

	// Validate detection settings
	if c.Detection.Confidence < 0 || c.Detection.Confidence > 1 {
		return fmt.Errorf("detection confidence must be between 0 and 1")
	}

	// Validate recognition settings
	if c.Recognition.SimilarityThreshold < 0 || c.Recognition.SimilarityThreshold > 1 {
		return fmt.Errorf("similarity threshold must be between 0 and 1")
	}

	// Validate auth settings
	if c.Auth.MaxAttempts <= 0 {
		return fmt.Errorf("max attempts must be positive")
	}

	return nil
}
