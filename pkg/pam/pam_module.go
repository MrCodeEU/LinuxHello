// Package main provides PAM module integration
package main

/*
#cgo LDFLAGS: -lpam -lpam_misc
#include <security/pam_appl.h>
#include <security/pam_modules.h>
#include <string.h>
#include <stdlib.h>

extern int pam_send_message(pam_handle_t *pamh, const char *message, int msg_style);
*/
import "C"

import (
	"context"
	"fmt"
	"os"
	"strconv"
	"strings"
	"time"
	"unsafe"

	"github.com/MrCodeEU/LinuxHello/internal/auth"
	"github.com/MrCodeEU/LinuxHello/internal/config"
	"github.com/MrCodeEU/LinuxHello/internal/embedding"
	"github.com/sirupsen/logrus"
)

var (
	logger *logrus.Logger
)

func init() {
	// Initialize logger with file output for debugging
	logger = logrus.New()
	logger.SetLevel(logrus.DebugLevel)

	// Try to write to a file for debugging PAM issues
	f, err := os.OpenFile("/var/log/linuxhello-pam.log", os.O_CREATE|os.O_WRONLY|os.O_APPEND, 0644)
	if err == nil {
		logger.SetOutput(f)
		logger.WithFields(logrus.Fields{
			"pid": os.Getpid(),
			"uid": os.Getuid(),
			"gid": os.Getgid(),
		}).Info("PAM module initialized with file logging")
	} else {
		logger.WithError(err).Warn("Failed to open PAM log file, using default output")
	}
}

// pamInfo sends an informational message to the user via PAM conversation
func pamInfo(pamh *C.pam_handle_t, msg string) {
	cMsg := C.CString(msg)
	defer C.free(unsafe.Pointer(cMsg))
	C.pam_send_message(pamh, cMsg, C.PAM_TEXT_INFO)
}

// pamError sends an error message to the user via PAM conversation
func pamError(pamh *C.pam_handle_t, msg string) {
	cMsg := C.CString(msg)
	defer C.free(unsafe.Pointer(cMsg))
	C.pam_send_message(pamh, cMsg, C.PAM_ERROR_MSG)
}

//export goAuthenticate
func goAuthenticate(pamh *C.pam_handle_t, _ C.int, argc C.int, argv **C.char) C.int {
	// Defensive check for logger
	if logger == nil {
		return C.PAM_AUTH_ERR
	}

	logger.Debug("goAuthenticate called")

	// Parse and validate arguments
	args := parseArgumentsSafely(argc, argv)
	logger.Debugf("args parsed: %v", args)

	// Load configuration
	cfg, err := loadConfig(args)
	if err != nil {
		logger.Errorf("Failed to load config: %v", err)
		return C.PAM_AUTH_ERR
	}

	// Get and validate username
	username, result := getUsernameWithValidation(pamh, cfg)
	if result != C.PAM_SUCCESS {
		return result
	}

	pamInfo(pamh, "LinuxHello: Authenticating...")

	// Initialize authentication system
	engine, result := initializeAuthEngine(cfg)
	if result != C.PAM_SUCCESS {
		pamError(pamh, "LinuxHello: Service unavailable")
		return result
	}
	defer func() { _ = engine.Close() }()

	// Initialize and start camera
	if result := setupCamera(engine, cfg); result != C.PAM_SUCCESS {
		pamError(pamh, "LinuxHello: Camera unavailable")
		return result
	}

	// Perform authentication
	return performAuthentication(pamh, engine, cfg, username)
}

// parseArgumentsSafely safely parses PAM arguments
func parseArgumentsSafely(argc C.int, argv **C.char) map[string]string {
	if argc > 0 && argv != nil {
		return parseArgs(argc, argv)
	}
	return make(map[string]string)
}

// getUsernameWithValidation gets and validates the username
func getUsernameWithValidation(pamh *C.pam_handle_t, cfg *config.Config) (string, C.int) {
	username, err := getUser(pamh)
	if err != nil {
		logger.Errorf("Failed to get username: %v", err)
		return "", C.PAM_AUTH_ERR
	}

	logger.Infof("Authenticating user: %s", username)

	// Check if user is enrolled
	if !isUserEnrolled(cfg, username) {
		logger.Warnf("User %s not enrolled in facelock", username)
		if cfg.Auth.FallbackEnabled {
			return "", C.PAM_IGNORE
		}
		return "", C.PAM_USER_UNKNOWN
	}

	return username, C.PAM_SUCCESS
}

// initializeAuthEngine initializes the authentication engine
func initializeAuthEngine(cfg *config.Config) (*auth.Engine, C.int) {
	engine, err := auth.NewEngine(cfg, logger)
	if err != nil {
		logger.Errorf("Failed to initialize engine: %v", err)
		if cfg.Auth.FallbackEnabled {
			return nil, C.PAM_IGNORE
		}
		return nil, C.PAM_AUTH_ERR
	}
	return engine, C.PAM_SUCCESS
}

// setupCamera initializes and starts the camera
func setupCamera(engine *auth.Engine, cfg *config.Config) C.int {
	// Initialize camera
	if err := engine.InitializeCamera(); err != nil {
		logger.Errorf("Failed to initialize camera: %v", err)
		return fallbackOrError(cfg)
	}

	// Start capture
	if err := engine.Start(); err != nil {
		logger.Errorf("Failed to start camera: %v", err)
		return fallbackOrError(cfg)
	}

	return C.PAM_SUCCESS
}

// performAuthentication executes the authentication process
func performAuthentication(pamh *C.pam_handle_t, engine *auth.Engine, cfg *config.Config, username string) C.int {
	ctx, cancel := context.WithTimeout(context.Background(),
		time.Duration(cfg.Auth.SessionTimeout)*time.Second)
	defer cancel()

	result, err := engine.AuthenticateUser(ctx, username)
	if err != nil {
		logger.Errorf("Authentication error: %v", err)
		pamError(pamh, "LinuxHello: Authentication error")
		return fallbackOrError(cfg)
	}

	if result.Success {
		logger.Infof("Authentication successful for user %s (confidence: %.3f, time: %v)",
			username, result.Confidence, result.ProcessingTime)
		pamInfo(pamh, fmt.Sprintf("LinuxHello: Authenticated as %s", username))
		return C.PAM_SUCCESS
	}

	logger.Warnf("Authentication failed for user %s: %v", username, result.Error)
	pamError(pamh, "LinuxHello: Authentication failed")
	return fallbackOrError(cfg)
}

// fallbackOrError returns appropriate PAM result based on fallback configuration
func fallbackOrError(cfg *config.Config) C.int {
	if cfg.Auth.FallbackEnabled {
		return C.PAM_IGNORE
	}
	return C.PAM_AUTH_ERR
}

// parseArgs parses PAM module arguments
func parseArgs(argc C.int, argv **C.char) map[string]string {
	args := make(map[string]string)

	// Convert C array to Go slice
	argvSlice := (*[1 << 30]*C.char)(unsafe.Pointer(argv))[:argc:argc]

	for i := 0; i < int(argc); i++ {
		arg := C.GoString(argvSlice[i])

		// Parse key=value or just key
		if idx := strings.Index(arg, "="); idx > 0 {
			key := arg[:idx]
			value := arg[idx+1:]
			args[key] = value
		} else {
			args[arg] = "true"
		}
	}

	return args
}

// loadConfig loads configuration with optional overrides from args
func loadConfig(args map[string]string) (*config.Config, error) {
	configPath := args["config"]

	cfg, err := config.Load(configPath)
	if err != nil {
		// Use default config if loading fails
		cfg = config.DefaultConfig()
	}

	// Apply argument overrides
	if device, ok := args["device"]; ok {
		cfg.Camera.Device = device
	}

	if threshold, ok := args["threshold"]; ok {
		if t, err := strconv.ParseFloat(threshold, 64); err == nil {
			cfg.Recognition.SimilarityThreshold = t
		}
	}

	if fallback, ok := args["fallback"]; ok {
		cfg.Auth.FallbackEnabled = fallback == "true" || fallback == "yes"
	}

	if timeout, ok := args["timeout"]; ok {
		if t, err := strconv.Atoi(timeout); err == nil {
			cfg.Auth.SessionTimeout = t
		}
	}

	return cfg, nil
}

// getUser retrieves the username from PAM
func getUser(pamh *C.pam_handle_t) (string, error) {
	var cUsername *C.char

	ret := C.pam_get_user(pamh, &cUsername, nil)
	if ret != C.PAM_SUCCESS {
		return "", fmt.Errorf("pam_get_user failed: %d", ret)
	}

	return C.GoString(cUsername), nil
}

// isUserEnrolled checks if a user has enrolled face data
func isUserEnrolled(cfg *config.Config, username string) bool {
	// Quick check without initializing full engine
	store, err := embedding.NewStore(cfg.Storage.DatabasePath)
	if err != nil {
		return false
	}
	defer func() { _ = store.Close() }()

	_, err = store.GetUser(username)
	return err == nil
}

// Main function required for c-shared buildmode
func main() {
	// This function is required for buildmode=c-shared but won't be called
	// The actual PAM functions are exported via CGO
}
