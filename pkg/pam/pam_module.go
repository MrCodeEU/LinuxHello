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
	"errors"
	"fmt"
	"os"
	"os/signal"
	"strconv"
	"strings"
	"syscall"
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

// setupSignalHandler creates a context that cancels on SIGINT/SIGTERM
// This allows Ctrl+C to cancel the face detection loop
func setupSignalHandler(parentCtx context.Context) (context.Context, context.CancelFunc) {
	ctx, cancel := context.WithCancel(parentCtx)

	sigChan := make(chan os.Signal, 1)
	signal.Notify(sigChan, syscall.SIGINT, syscall.SIGTERM)

	go func() {
		select {
		case sig := <-sigChan:
			logger.Infof("Received signal %v, cancelling authentication", sig)
			cancel()
		case <-ctx.Done():
		}
		signal.Stop(sigChan)
	}()

	return ctx, cancel
}

// performAuthentication executes the authentication process.
// Provides Windows Hello-like real-time status messages via PAM conversation.
// The authentication will wait for a face to be detected (with optional timeout),
// and can be cancelled via Ctrl+C (SIGINT) or the context.
func performAuthentication(pamh *C.pam_handle_t, engine *auth.Engine, cfg *config.Config, username string) C.int {
	// Create base context
	ctx := context.Background()

	// Setup signal handling for Ctrl+C support
	ctx, signalCancel := setupSignalHandler(ctx)
	defer signalCancel()

	// Apply optional timeout for face detection (useful for graphical login managers)
	// A value of 0 means no timeout (wait indefinitely)
	if cfg.Auth.FaceDetectionTimeout > 0 {
		var timeoutCancel context.CancelFunc
		ctx, timeoutCancel = context.WithTimeout(ctx, time.Duration(cfg.Auth.FaceDetectionTimeout)*time.Second)
		defer timeoutCancel()
		logger.Debugf("Face detection timeout set to %d seconds", cfg.Auth.FaceDetectionTimeout)
	}

	// Create status channel for real-time Windows Hello-like feedback.
	// Buffer of 32 to absorb bursts during rapid retry loops; sendStatus drops
	// updates non-blocking if the goroutine falls behind (e.g. PAM conversation blocks).
	statusChan := make(chan auth.StatusUpdate, 32)

	// Relay status updates to PAM conversation in a separate goroutine
	statusDone := make(chan struct{})
	go func() {
		defer close(statusDone)
		lastMsg := ""
		for update := range statusChan {
			if !cfg.Auth.ShowStatusMessages {
				continue
			}
			msg := "LinuxHello: " + update.Message
			// Avoid sending duplicate consecutive messages (e.g. repeated "Looking for you...")
			if msg == lastMsg {
				continue
			}
			lastMsg = msg
			switch update.Status {
			case auth.StatusSuccess:
				pamInfo(pamh, msg)
			case auth.StatusLivenessHint, auth.StatusNoMatch:
				pamError(pamh, msg)
			case auth.StatusFallback:
				pamError(pamh, msg) // security-relevant: biometric lockout or max attempts exceeded
			default:
				pamInfo(pamh, msg)
			}
		}
	}()

	result, err := engine.AuthenticateUser(ctx, username, statusChan)
	close(statusChan)
	<-statusDone // Wait for all status messages to be sent

	if err != nil {
		// Check if cancelled by user (Ctrl+C)
		if ctx.Err() == context.Canceled {
			logger.Info("Authentication cancelled by user (Ctrl+C)")
			return C.PAM_AUTH_ERR
		}
		// Check if timed out
		if ctx.Err() == context.DeadlineExceeded {
			logger.Info("Face detection timed out")
			pamInfo(pamh, "LinuxHello: Face detection timed out")
			return fallbackOrError(cfg)
		}
		logger.Errorf("Authentication error: %v", err)
		pamError(pamh, "LinuxHello: Authentication error")
		return fallbackOrError(cfg)
	}

	if result.Success {
		logger.Infof("Authentication successful for user %s (confidence: %.3f, time: %v)",
			username, result.Confidence, result.ProcessingTime)
		return C.PAM_SUCCESS
	}

	// Handle specific failure modes
	if result.Error != nil {
		switch {
		case errors.Is(result.Error, auth.ErrAccountLocked):
			logger.Warnf("Account lockout active for user %s: %v", username, result.Error)
			pamError(pamh, "LinuxHello: Too many failed attempts. Use password.")
			return fallbackOrError(cfg)

		case errors.Is(result.Error, auth.ErrAuthenticationCancelled):
			if ctx.Err() == context.DeadlineExceeded {
				logger.Info("Face detection timed out")
				pamInfo(pamh, "LinuxHello: Face detection timed out")
				return fallbackOrError(cfg)
			}
			logger.Info("Authentication cancelled by user (Ctrl+C)")
			return C.PAM_AUTH_ERR

		case errors.Is(result.Error, auth.ErrBiometricLockout):
			// Too many liveness failures - fall back to password
			logger.Warn("Biometric lockout: falling back to password authentication")
			return fallbackOrError(cfg)

		case errors.Is(result.Error, auth.ErrMaxAttemptsExceeded):
			// Too many face match attempts - fall back to password
			logger.Warn("Max face auth attempts exceeded: falling back to password authentication")
			return fallbackOrError(cfg)
		}
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
