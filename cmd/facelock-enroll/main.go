// facelock-enroll - Face enrollment tool for FaceLock
package main

import (
	"flag"
	"fmt"
	"os"
	"strings"

	"github.com/facelock/facelock/internal/auth"
	"github.com/facelock/facelock/internal/config"
	"github.com/facelock/facelock/internal/embedding"
	"github.com/sirupsen/logrus"
)

func main() {
	var (
		username   = flag.String("user", "", "Username to enroll")
		numSamples = flag.Int("samples", 5, "Number of face samples to capture")
		configPath = flag.String("config", "", "Path to configuration file")
		deleteUser = flag.String("delete", "", "Delete user enrollment")
		listUsers  = flag.Bool("list", false, "List enrolled users")
		verbose    = flag.Bool("verbose", false, "Enable verbose output")
		debug      = flag.Bool("debug", false, "Save debug images of enrollment samples")
	)
	flag.Parse()

	// Setup logger
	logger := logrus.New()
	if *verbose {
		logger.SetLevel(logrus.DebugLevel)
	} else {
		logger.SetLevel(logrus.InfoLevel)
	}

	// Load configuration
	cfg, err := config.Load(*configPath)
	if err != nil {
		logger.Warnf("Using default configuration: %v", err)
		cfg = config.DefaultConfig()
	}

	// Handle list command
	if *listUsers {
		if err := listEnrolledUsers(cfg, logger); err != nil {
			logger.Fatalf("Failed to list users: %v", err)
		}
		return
	}

	// Handle delete command
	if *deleteUser != "" {
		if err := deleteUserEnrollment(cfg, *deleteUser, logger); err != nil {
			logger.Fatalf("Failed to delete user: %v", err)
		}
		return
	}

	// Validate username for enrollment
	if *username == "" {
		fmt.Println("Usage: facelock-enroll -user <username> [options]")
		fmt.Println()
		fmt.Println("Options:")
		flag.PrintDefaults()
		fmt.Println()
		fmt.Println("Examples:")
		fmt.Println("  facelock-enroll -user john                    # Enroll user 'john'")
		fmt.Println("  facelock-enroll -user john -samples 10        # Enroll with 10 samples")
		fmt.Println("  facelock-enroll -list                         # List all enrolled users")
		fmt.Println("  facelock-enroll -delete john                  # Delete user 'john'")
		os.Exit(1)
	}

	// Validate username
	if !isValidUsername(*username) {
		logger.Fatalf("Invalid username: %s", *username)
	}

	// Perform enrollment
	if err := enrollUser(cfg, *username, *numSamples, *debug, logger); err != nil {
		logger.Fatalf("Enrollment failed: %v", err)
	}
}

func enrollUser(cfg *config.Config, username string, numSamples int, debug bool, logger *logrus.Logger) error {
	fmt.Printf("FaceLock Enrollment\n")
	fmt.Printf("===================\n\n")
	fmt.Printf("User: %s\n", username)
	fmt.Printf("Samples: %d\n\n", numSamples)

	// Create authentication engine
	engine, err := auth.NewEngine(cfg, logger)
	if err != nil {
		return fmt.Errorf("failed to create engine: %w", err)
	}
	defer func() { _ = engine.Close() }()

	// Initialize camera
	fmt.Println("Initializing camera...")
	if err := engine.InitializeCamera(); err != nil {
		return fmt.Errorf("failed to initialize camera: %w", err)
	}

	// Start capture
	if err := engine.Start(); err != nil {
		return fmt.Errorf("failed to start camera: %w", err)
	}

	fmt.Println("Camera ready.")

	// Check if user already exists
	store := engine.GetEmbeddingStore()
	existingUser, err := store.GetUser(username)
	if err == nil {
		fmt.Printf("User '%s' already exists with %d enrollment samples.\n",
			username, len(existingUser.Embeddings))
		fmt.Print("Do you want to update enrollment? [y/N]: ")

		var response string
		_, _ = fmt.Scanln(&response)

		if strings.ToLower(response) != "y" && strings.ToLower(response) != "yes" {
			fmt.Println("Enrollment cancelled.")
			return nil
		}

		// Delete existing enrollment
		if err := store.DeleteUser(username); err != nil {
			return fmt.Errorf("failed to delete existing enrollment: %w", err)
		}
		fmt.Println("Existing enrollment removed.")
	}

	// Instructions
	fmt.Println("Enrollment Instructions:")
	fmt.Println("------------------------")
	fmt.Println("1. Position your face in front of the camera")
	fmt.Println("2. Ensure good lighting on your face")
	fmt.Println("3. Look directly at the camera")
	fmt.Println("4. Keep your face steady during capture")
	fmt.Println("5. Vary your position slightly between samples")
	fmt.Println()

	// Wait for user to be ready
	fmt.Print("Press Enter when ready to start enrollment...")
	_, _ = fmt.Scanln()
	fmt.Println()

	var debugDir string
	if debug {
		debugDir = "debug_enrollment"
		fmt.Println("Debug mode enabled: saving samples to debug_enrollment/")
	}

	// Perform enrollment
	user, err := engine.EnrollUser(username, numSamples, debugDir)
	if err != nil {
		return fmt.Errorf("enrollment failed: %w", err)
	}

	fmt.Println()
	fmt.Println("Enrollment Successful!")
	fmt.Println("=====================")
	fmt.Printf("User ID: %s\n", user.ID)
	fmt.Printf("Username: %s\n", user.Username)
	fmt.Printf("Samples captured: %d\n", len(user.Embeddings))
	fmt.Printf("Enrollment time: %s\n", user.CreatedAt.Format("2006-01-02 15:04:05"))
	fmt.Println()
	fmt.Println("You can now use face authentication for this user.")

	return nil
}

func listEnrolledUsers(cfg *config.Config, logger *logrus.Logger) error {
	// Open store directly
	store, err := embedding.NewStore(cfg.Storage.DatabasePath)
	if err != nil {
		return fmt.Errorf("failed to open store: %w", err)
	}
	defer func() { _ = store.Close() }()

	users, err := store.ListUsers()
	if err != nil {
		return fmt.Errorf("failed to list users: %w", err)
	}

	if len(users) == 0 {
		fmt.Println("No enrolled users found.")
		return nil
	}

	fmt.Println("Enrolled Users")
	fmt.Println("==============")
	fmt.Printf("%-20s %-10s %-20s %-10s\n", "Username", "Samples", "Last Used", "Use Count")
	fmt.Println(strings.Repeat("-", 65))

	for _, user := range users {
		lastUsed := "Never"
		if user.LastUsedAt != nil {
			lastUsed = user.LastUsedAt.Format("2006-01-02")
		}

		status := "Active"
		if !user.Active {
			status = "Inactive"
		}

		fmt.Printf("%-20s %-10d %-20s %-10d (%s)\n",
			user.Username, len(user.Embeddings), lastUsed, user.UseCount, status)
	}

	fmt.Printf("\nTotal: %d user(s)\n", len(users))

	return nil
}

func deleteUserEnrollment(cfg *config.Config, username string, logger *logrus.Logger) error {
	// Open store directly
	store, err := embedding.NewStore(cfg.Storage.DatabasePath)
	if err != nil {
		return fmt.Errorf("failed to open store: %w", err)
	}
	defer func() { _ = store.Close() }()

	// Check if user exists
	_, err = store.GetUser(username)
	if err != nil {
		return fmt.Errorf("user not found: %s", username)
	}

	// Confirm deletion
	fmt.Printf("Are you sure you want to delete enrollment for user '%s'? [y/N]: ", username)

	var response string
	_, _ = fmt.Scanln(&response)

	if strings.ToLower(response) != "y" && strings.ToLower(response) != "yes" {
		fmt.Println("Deletion cancelled.")
		return nil
	}

	// Delete user
	if err := store.DeleteUser(username); err != nil {
		return fmt.Errorf("failed to delete user: %w", err)
	}

	fmt.Printf("User '%s' enrollment deleted successfully.\n", username)

	return nil
}

func isValidUsername(username string) bool {
	if username == "" {
		return false
	}

	// Check for valid characters
	for _, c := range username {
		isLower := c >= 'a' && c <= 'z'
		isUpper := c >= 'A' && c <= 'Z'
		isDigit := c >= '0' && c <= '9'
		isSpecial := c == '_' || c == '-' || c == '.'
		
		if !isLower && !isUpper && !isDigit && !isSpecial {
			return false
		}
	}

	return true
}
