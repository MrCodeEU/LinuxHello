// Package config provides configuration management for LinuxHello
package config

import (
	"fmt"
	"os"
	"path/filepath"

	"github.com/spf13/viper"
	"gopkg.in/yaml.v3"
)

// Config holds all configuration for the application
type Config struct {
	// Inference service settings
	Inference InferenceConfig `mapstructure:"inference" json:"inference" yaml:"inference"`

	// Camera settings
	Camera CameraConfig `mapstructure:"camera" json:"camera" yaml:"camera"`

	// Detection settings
	Detection DetectionConfig `mapstructure:"detection" json:"detection" yaml:"detection"`

	// Recognition settings
	Recognition RecognitionConfig `mapstructure:"recognition" json:"recognition" yaml:"recognition"`

	// Liveness settings
	Liveness LivenessConfig `mapstructure:"liveness" json:"liveness" yaml:"liveness"`

	// Challenge-response settings
	Challenge ChallengeConfig `mapstructure:"challenge" json:"challenge" yaml:"challenge"`

	// Lockout system settings
	Lockout LockoutConfig `mapstructure:"lockout" json:"lockout" yaml:"lockout"`

	// Authentication settings
	Auth AuthConfig `mapstructure:"auth" json:"auth" yaml:"auth"`

	// Storage settings
	Storage StorageConfig `mapstructure:"storage" json:"storage" yaml:"storage"`

	// Logging settings
	Logging LoggingConfig `mapstructure:"logging" json:"logging" yaml:"logging"`
}

// InferenceConfig holds inference service configuration
type InferenceConfig struct {
	Address string `mapstructure:"address" json:"address" yaml:"address"`
	Timeout int    `mapstructure:"timeout" json:"timeout" yaml:"timeout"`
}

// CameraConfig holds camera-related configuration
type CameraConfig struct {
	Device       string `mapstructure:"device" json:"device" yaml:"device"`
	IRDevice     string `mapstructure:"ir_device" json:"ir_device" yaml:"ir_device"`
	DepthDevice  string `mapstructure:"depth_device" json:"depth_device" yaml:"depth_device"`
	Width        int    `mapstructure:"width" json:"width" yaml:"width"`
	Height       int    `mapstructure:"height" json:"height" yaml:"height"`
	FPS          int    `mapstructure:"fps" json:"fps" yaml:"fps"`
	PixelFormat  string `mapstructure:"pixel_format" json:"pixel_format" yaml:"pixel_format"`
	UseRealSense bool   `mapstructure:"use_realsense" json:"use_realsense" yaml:"use_realsense"`
	AutoExposure bool   `mapstructure:"auto_exposure" json:"auto_exposure" yaml:"auto_exposure"`
}

// DetectionConfig holds face detection configuration
type DetectionConfig struct {
	ModelPath     string  `mapstructure:"model_path" json:"model_path" yaml:"model_path"`
	Confidence    float32 `mapstructure:"confidence" json:"confidence" yaml:"confidence"`
	NMSThreshold  float32 `mapstructure:"nms_threshold" json:"nms_threshold" yaml:"nms_threshold"`
	InputSize     int     `mapstructure:"input_size" json:"input_size" yaml:"input_size"`
	MaxDetections int     `mapstructure:"max_detections" json:"max_detections" yaml:"max_detections"`
}

// RecognitionConfig holds face recognition configuration
type RecognitionConfig struct {
	ModelPath           string  `mapstructure:"model_path" json:"model_path" yaml:"model_path"`
	InputSize           int     `mapstructure:"input_size" json:"input_size" yaml:"input_size"`
	EmbeddingSize       int     `mapstructure:"embedding_size" json:"embedding_size" yaml:"embedding_size"`
	SimilarityThreshold float64 `mapstructure:"similarity_threshold" json:"similarity_threshold" yaml:"similarity_threshold"`
	EnrollmentSamples   int     `mapstructure:"enrollment_samples" json:"enrollment_samples" yaml:"enrollment_samples"`
}

// LivenessConfig holds liveness detection configuration
type LivenessConfig struct {
	Enabled             bool    `mapstructure:"enabled" json:"enabled" yaml:"enabled"`
	ModelPath           string  `mapstructure:"model_path" json:"model_path" yaml:"model_path"`
	DepthThreshold      float32 `mapstructure:"depth_threshold" json:"depth_threshold" yaml:"depth_threshold"`
	ConfidenceThreshold float32 `mapstructure:"confidence_threshold" json:"confidence_threshold" yaml:"confidence_threshold"`
	UseDepthCamera      bool    `mapstructure:"use_depth_camera" json:"use_depth_camera" yaml:"use_depth_camera"`
	UseIRAnalysis       bool    `mapstructure:"use_ir_analysis" json:"use_ir_analysis" yaml:"use_ir_analysis"`
}

// ChallengeConfig holds challenge-response configuration
type ChallengeConfig struct {
	Enabled         bool     `mapstructure:"enabled" json:"enabled" yaml:"enabled"`
	ChallengeTypes  []string `mapstructure:"challenge_types" json:"challenge_types" yaml:"challenge_types"`
	TimeoutSeconds  int      `mapstructure:"timeout_seconds" json:"timeout_seconds" yaml:"timeout_seconds"`
	RequiredSuccess int      `mapstructure:"required_success" json:"required_success" yaml:"required_success"`
}

// LockoutConfig holds account lockout configuration
type LockoutConfig struct {
	Enabled            bool `mapstructure:"enabled" json:"enabled" yaml:"enabled"`
	MaxFailures        int  `mapstructure:"max_failures" json:"max_failures" yaml:"max_failures"`
	LockoutDuration    int  `mapstructure:"lockout_duration" json:"lockout_duration" yaml:"lockout_duration"` // in minutes
	ProgressiveLockout bool `mapstructure:"progressive_lockout" json:"progressive_lockout" yaml:"progressive_lockout"`
}

// AuthConfig holds authentication configuration
type AuthConfig struct {
	MaxAttempts     int    `mapstructure:"max_attempts" json:"max_attempts" yaml:"max_attempts"`
	LockoutDuration int    `mapstructure:"lockout_duration" json:"lockout_duration" yaml:"lockout_duration"`
	SessionTimeout  int    `mapstructure:"session_timeout" json:"session_timeout" yaml:"session_timeout"`
	FallbackEnabled bool   `mapstructure:"fallback_enabled" json:"fallback_enabled" yaml:"fallback_enabled"`
	ContinuousAuth  bool   `mapstructure:"continuous_auth" json:"continuous_auth" yaml:"continuous_auth"`
	SecurityLevel   string `mapstructure:"security_level" json:"security_level" yaml:"security_level"`
}

// StorageConfig holds data storage configuration
type StorageConfig struct {
	DataDir       string `mapstructure:"data_dir" json:"data_dir" yaml:"data_dir"`
	DatabasePath  string `mapstructure:"database_path" json:"database_path" yaml:"database_path"`
	MaxUsers      int    `mapstructure:"max_users" json:"max_users" yaml:"max_users"`
	BackupEnabled bool   `mapstructure:"backup_enabled" json:"backup_enabled" yaml:"backup_enabled"`
}

// LoggingConfig holds logging configuration
type LoggingConfig struct {
	Level      string `mapstructure:"level" json:"level" yaml:"level"`
	File       string `mapstructure:"file" json:"file" yaml:"file"`
	MaxSize    int    `mapstructure:"max_size" json:"max_size" yaml:"max_size"`
	MaxBackups int    `mapstructure:"max_backups" json:"max_backups" yaml:"max_backups"`
	MaxAge     int    `mapstructure:"max_age" json:"max_age" yaml:"max_age"`
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
			Enabled:         false, // Disabled by default
			ChallengeTypes:  []string{"blink"},
			TimeoutSeconds:  10,
			RequiredSuccess: 1,
		},
		Lockout: LockoutConfig{
			Enabled:            false, // Disabled by default
			MaxFailures:        5,
			LockoutDuration:    15, // 15 minutes
			ProgressiveLockout: false,
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
			DataDir:       "/var/lib/linuxhello",
			DatabasePath:  "/var/lib/linuxhello/facelock.db",
			MaxUsers:      100,
			BackupEnabled: true,
		},
		Logging: LoggingConfig{
			Level:      "info",
			File:       "/var/log/linuxhello.log",
			MaxSize:    100,
			MaxBackups: 3,
			MaxAge:     30,
		},
	}
}

// Load loads configuration from file and environment variables
func Load(configPath string) (*Config, error) {
	cfg := DefaultConfig()

	v := viper.New()
	v.SetConfigType("yaml")

	if configPath != "" {
		v.SetConfigFile(configPath)
	} else {
		v.SetConfigName("facelock")
		v.AddConfigPath("/etc/linuxhello/")
		v.AddConfigPath("$HOME/.facelock")
		v.AddConfigPath(".")
	}

	v.SetEnvPrefix("FACELOCK")
	v.AutomaticEnv()

	if err := v.ReadInConfig(); err != nil {
		if _, ok := err.(viper.ConfigFileNotFoundError); !ok {
			return nil, fmt.Errorf("error reading config: %w", err)
		}
	}

	if err := v.Unmarshal(cfg); err != nil {
		return nil, fmt.Errorf("error unmarshaling config: %w", err)
	}

	if cfg.Storage.DataDir != "" {
		if err := os.MkdirAll(cfg.Storage.DataDir, 0755); err != nil {
			return nil, fmt.Errorf("error creating data directory: %w", err)
		}
	}

	return cfg, nil
}

// Save saves the configuration to a file using YAML format
func (c *Config) Save(path string) error {
	data, err := yaml.Marshal(c)
	if err != nil {
		return fmt.Errorf("failed to marshal config: %w", err)
	}

	dir := filepath.Dir(path)
	if err := os.MkdirAll(dir, 0755); err != nil {
		return fmt.Errorf("error creating config directory: %w", err)
	}

	if err := os.WriteFile(path, data, 0644); err != nil {
		return fmt.Errorf("error writing config to %s: %w", path, err)
	}

	return nil
}

// Validate validates the configuration
func (c *Config) Validate() error {
	if c.Camera.Device == "" {
		return fmt.Errorf("camera device cannot be empty")
	}
	if c.Camera.Width <= 0 || c.Camera.Height <= 0 {
		return fmt.Errorf("invalid camera resolution: %dx%d", c.Camera.Width, c.Camera.Height)
	}
	if c.Detection.Confidence < 0 || c.Detection.Confidence > 1 {
		return fmt.Errorf("detection confidence must be between 0 and 1")
	}
	if c.Recognition.SimilarityThreshold < 0 || c.Recognition.SimilarityThreshold > 1 {
		return fmt.Errorf("similarity threshold must be between 0 and 1")
	}
	if c.Auth.MaxAttempts <= 0 {
		return fmt.Errorf("max attempts must be positive")
	}
	return nil
}
