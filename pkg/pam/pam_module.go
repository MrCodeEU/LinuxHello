// Package pam provides PAM module integration
package pam

/*
#cgo LDFLAGS: -lpam -lpam_misc
#include <security/pam_appl.h>
#include <security/pam_modules.h>
#include <string.h>
#include <stdlib.h>

// Forward declaration of Go function
extern int goAuthenticate(pam_handle_t *pamh, int flags, int argc, char **argv);

// PAM service function that calls Go
int pam_sm_authenticate(pam_handle_t *pamh, int flags, int argc, const char **argv) {
    return goAuthenticate(pamh, flags, argc, (char**)argv);
}

int pam_sm_setcred(pam_handle_t *pamh, int flags, int argc, const char **argv) {
    return PAM_SUCCESS;
}

int pam_sm_acct_mgmt(pam_handle_t *pamh, int flags, int argc, const char **argv) {
    return PAM_SUCCESS;
}

int pam_sm_open_session(pam_handle_t *pamh, int flags, int argc, const char **argv) {
    return PAM_SUCCESS;
}

int pam_sm_close_session(pam_handle_t *pamh, int flags, int argc, const char **argv) {
    return PAM_SUCCESS;
}

int pam_sm_chauthtok(pam_handle_t *pamh, int flags, int argc, const char **argv) {
    return PAM_SERVICE_ERR;
}
*/
import "C"

import (
	"context"
	"fmt"
	"strconv"
	"strings"
	"time"
	"unsafe"

	"github.com/facelock/facelock/internal/auth"
	"github.com/facelock/facelock/internal/config"
	"github.com/facelock/facelock/internal/embedding"
	"github.com/sirupsen/logrus"
)

var (
	logger *logrus.Logger
)

func init() {
	// Initialize logger
	logger = logrus.New()
	logger.SetLevel(logrus.InfoLevel)
}

//export goAuthenticate
func goAuthenticate(pamh *C.pam_handle_t, flags C.int, argc C.int, argv **C.char) C.int {
	// Parse arguments
	args := parseArgs(argc, argv)

	// Load configuration
	cfg, err := loadConfig(args)
	if err != nil {
		logger.Errorf("Failed to load config: %v", err)
		return C.PAM_AUTH_ERR
	}

	// Get username from PAM
	username, err := getUser(pamh)
	if err != nil {
		logger.Errorf("Failed to get username: %v", err)
		return C.PAM_AUTH_ERR
	}

	logger.Infof("Authenticating user: %s", username)

	// Check if user is enrolled
	if !isUserEnrolled(cfg, username) {
		logger.Warnf("User %s not enrolled in facelock", username)
		// Return success if fallback is enabled, allowing other PAM modules to handle it
		if cfg.Auth.FallbackEnabled {
			return C.PAM_IGNORE
		}
		return C.PAM_USER_UNKNOWN
	}

	// Initialize authentication engine
	engine, err := auth.NewEngine(cfg, logger)
	if err != nil {
		logger.Errorf("Failed to initialize engine: %v", err)
		if cfg.Auth.FallbackEnabled {
			return C.PAM_IGNORE
		}
		return C.PAM_AUTH_ERR
	}
	defer func() { _ = engine.Close() }()

	// Initialize camera
	if err := engine.InitializeCamera(); err != nil {
		logger.Errorf("Failed to initialize camera: %v", err)
		if cfg.Auth.FallbackEnabled {
			return C.PAM_IGNORE
		}
		return C.PAM_AUTH_ERR
	}

	// Start capture
	if err := engine.Start(); err != nil {
		logger.Errorf("Failed to start camera: %v", err)
		if cfg.Auth.FallbackEnabled {
			return C.PAM_IGNORE
		}
		return C.PAM_AUTH_ERR
	}

	// Perform authentication
	ctx, cancel := context.WithTimeout(context.Background(),
		time.Duration(cfg.Auth.SessionTimeout)*time.Second)
	defer cancel()

	result, err := engine.AuthenticateUser(ctx, username)
	if err != nil {
		logger.Errorf("Authentication error: %v", err)
		if cfg.Auth.FallbackEnabled {
			return C.PAM_IGNORE
		}
		return C.PAM_AUTH_ERR
	}

	if result.Success {
		logger.Infof("Authentication successful for user %s (confidence: %.3f, time: %v)",
			username, result.Confidence, result.ProcessingTime)
		return C.PAM_SUCCESS
	}

	logger.Warnf("Authentication failed for user %s: %v", username, result.Error)

	// Check if we should allow fallback
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

// Helper function for sending messages to user (for future use)
func sendMessage(pamh *C.pam_handle_t, msg string) error {
	cMsg := C.CString(msg)
	defer C.free(unsafe.Pointer(cMsg))

	msgStyle := C.int(C.PAM_TEXT_INFO)
	pamMsg := C.struct_pam_message{
		msg_style: msgStyle,
		msg:       cMsg,
	}

	// This is a simplified version - full implementation would use pam_conv
	_ = pamMsg

	return nil
}

// Helper function for getting user input (for future use)
func getInput(pamh *C.pam_handle_t, prompt string) (string, error) {
	// This would use pam_conv to get user input
	// For now, return empty string
	return "", nil
}
