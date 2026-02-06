package cli

import (
	"context"
	"flag"
	"fmt"
	"os"
	"os/signal"
	"syscall"
	"time"

	"github.com/MrCodeEU/LinuxHello/internal/auth"
	"github.com/MrCodeEU/LinuxHello/internal/config"
	"github.com/sirupsen/logrus"
)

// RunTest runs the authentication test CLI
func RunTest(args []string) {
	fs := flag.NewFlagSet("test", flag.ExitOnError)
	username := fs.String("user", "", "Specific user to authenticate (optional)")
	configPath := fs.String("config", "", "Path to configuration file")
	verbose := fs.Bool("verbose", false, "Enable verbose output")
	continuous := fs.Bool("continuous", false, "Continuous authentication mode")
	showFPS := fs.Bool("fps", false, "Show frames per second")
	_ = fs.Parse(args)

	logger := logrus.New()
	if *verbose {
		logger.SetLevel(logrus.DebugLevel)
	} else {
		logger.SetLevel(logrus.InfoLevel)
	}

	cfg, err := config.Load(*configPath)
	if err != nil {
		logger.Warnf("Using default configuration: %v", err)
		cfg = config.DefaultConfig()
	}

	if *continuous {
		if err := runContinuousMode(cfg, *username, *showFPS, logger); err != nil {
			logger.Fatalf("Continuous mode failed: %v", err)
		}
		return
	}

	if err := runSingleAuth(cfg, *username, logger); err != nil {
		logger.Fatalf("Authentication test failed: %v", err)
	}
}

func runSingleAuth(cfg *config.Config, username string, logger *logrus.Logger) error {
	fmt.Println("LinuxHello Authentication Test")
	fmt.Println("===========================")

	engine, err := auth.NewEngine(cfg, logger)
	if err != nil {
		return fmt.Errorf("failed to create engine: %w", err)
	}
	defer func() {
		if err := engine.Close(); err != nil {
			logger.Errorf("Failed to close engine: %v", err)
		}
	}()

	fmt.Println("Initializing camera...")
	if err := engine.InitializeCamera(); err != nil {
		return fmt.Errorf("failed to initialize camera: %w", err)
	}

	if err := engine.Start(); err != nil {
		return fmt.Errorf("failed to start camera: %w", err)
	}

	fmt.Println("Camera ready.")

	fmt.Println("Instructions:")
	fmt.Println("-------------")
	fmt.Println("1. Position your face in front of the camera")
	fmt.Println("2. Look directly at the camera")
	fmt.Println("3. Keep your face steady")
	fmt.Println()

	fmt.Print("Press Enter to start authentication...")
	_, _ = fmt.Scanln()
	fmt.Println()

	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()

	fmt.Println("Authenticating...")
	fmt.Println()

	var result *auth.Result
	if username != "" {
		result, err = engine.AuthenticateUser(ctx, username)
	} else {
		result, err = engine.Authenticate(ctx)
	}

	if err != nil {
		return fmt.Errorf("authentication error: %w", err)
	}

	fmt.Println()
	fmt.Println("Authentication Result")
	fmt.Println("====================")
	fmt.Printf("Success: %v\n", result.Success)

	if result.Success {
		fmt.Printf("User: %s\n", result.User.Username)
		fmt.Printf("Confidence: %.3f\n", result.Confidence)
		fmt.Printf("Liveness check: %v\n", result.LivenessPassed)
		fmt.Printf("Challenge check: %v\n", result.ChallengePassed)
	} else {
		if result.Error != nil {
			fmt.Printf("Error: %v\n", result.Error)
		}
		if result.Confidence > 0 {
			fmt.Printf("Best match confidence: %.3f\n", result.Confidence)
		}
	}

	fmt.Printf("Processing time: %v\n", result.ProcessingTime)

	if result.Success {
		fmt.Println("\n✓ Authentication successful!")
		return nil
	}

	fmt.Println("\n✗ Authentication failed")
	return fmt.Errorf("authentication failed")
}

func runContinuousMode(cfg *config.Config, username string, showFPS bool, logger *logrus.Logger) error {
	engine, err := setupAuthenticationEngine(cfg, logger)
	if err != nil {
		return err
	}
	defer func() { _ = engine.Close() }()

	fmt.Println("Camera ready. Starting continuous authentication...")

	sigChan := make(chan os.Signal, 1)
	signal.Notify(sigChan, syscall.SIGINT, syscall.SIGTERM)

	stats := &sessionStats{lastFPSUpdate: time.Now()}

	ticker := time.NewTicker(100 * time.Millisecond)
	defer ticker.Stop()

	for {
		select {
		case <-sigChan:
			fmt.Println("\n\nExiting...")
			printStats(stats.attempts, stats.successes, stats.failures, stats.totalTime)
			return nil

		case <-ticker.C:
			processContinuousFrame(engine, username, showFPS, stats)
		}
	}
}

type sessionStats struct {
	attempts        int
	successes       int
	failures        int
	totalTime       time.Duration
	lastFPSUpdate   time.Time
	framesProcessed int
}

func setupAuthenticationEngine(cfg *config.Config, logger *logrus.Logger) (*auth.Engine, error) {
	fmt.Println("LinuxHello Continuous Authentication Mode")
	fmt.Println("=======================================")
	fmt.Println("Press Ctrl+C to exit")

	engine, err := auth.NewEngine(cfg, logger)
	if err != nil {
		return nil, fmt.Errorf("failed to create engine: %w", err)
	}

	fmt.Println("Initializing camera...")
	if err := engine.InitializeCamera(); err != nil {
		_ = engine.Close()
		return nil, fmt.Errorf("failed to initialize camera: %w", err)
	}

	if err := engine.Start(); err != nil {
		_ = engine.Close()
		return nil, fmt.Errorf("failed to start camera: %w", err)
	}
	return engine, nil
}

func processContinuousFrame(engine *auth.Engine, username string, showFPS bool, stats *sessionStats) {
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	var result *auth.Result
	var err error

	if username != "" {
		result, err = engine.AuthenticateUser(ctx, username)
	} else {
		result, err = engine.Authenticate(ctx)
	}

	stats.attempts++
	if result != nil {
		stats.totalTime += result.ProcessingTime
	}

	if result != nil && result.Success {
		stats.successes++
		fmt.Printf("[✓] User: %s | Confidence: %.3f | Time: %v\n",
			result.User.Username, result.Confidence, result.ProcessingTime)
	} else {
		stats.failures++
		if stats.attempts%10 == 0 {
			if result != nil {
				fmt.Printf("[✗] Failed | Time: %v\n", result.ProcessingTime)
			} else {
				fmt.Printf("[✗] Failed | Error: %v\n", err)
			}
		}
	}

	stats.framesProcessed++

	if showFPS && time.Since(stats.lastFPSUpdate) >= time.Second {
		fps := float64(stats.framesProcessed) / time.Since(stats.lastFPSUpdate).Seconds()
		fmt.Printf("[FPS: %.1f] ", fps)
		stats.lastFPSUpdate = time.Now()
		stats.framesProcessed = 0
	}
}

func printStats(attempts, successes, failures int, totalTime time.Duration) {
	fmt.Println("\nSession Statistics")
	fmt.Println("==================")
	fmt.Printf("Total attempts: %d\n", attempts)

	if attempts > 0 {
		fmt.Printf("Successful: %d (%.1f%%)\n", successes, float64(successes)/float64(attempts)*100)
		fmt.Printf("Failed: %d (%.1f%%)\n", failures, float64(failures)/float64(attempts)*100)
		avgTime := totalTime / time.Duration(attempts)
		fmt.Printf("Average processing time: %v\n", avgTime)
	}
}
