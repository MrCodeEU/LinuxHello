package main

import (
	"context"
	"flag"
	"fmt"
	"net"
	"os"
	"os/signal"
	"syscall"
	"time"

	"github.com/facelock/facelock/internal/auth"
	"github.com/facelock/facelock/internal/config"
	"github.com/sirupsen/logrus"
)

func main() {
	var (
		configPath = flag.String("config", "/etc/facelock/facelock.conf", "Path to configuration file")
		verbose    = flag.Bool("verbose", false, "Enable verbose logging")
		version    = flag.Bool("version", false, "Show version information")
		daemon     = flag.Bool("daemon", false, "Run as daemon (background service)")
	)
	flag.Parse()

	// Version info
	if *version {
		printVersion()
		return
	}

	// Setup logger
	logger := logrus.New()
	if *verbose {
		logger.SetLevel(logrus.DebugLevel)
	} else {
		logger.SetLevel(logrus.InfoLevel)
	}

	// Load configuration
	cfg := loadConfiguration(*configPath, logger)

	// Validate configuration
	if err := cfg.Validate(); err != nil {
		logger.Fatalf("Invalid configuration: %v", err)
	}

	// Setup signal handling
	ctx, cancel := setupSignalHandling(logger, *configPath, &cfg)
	defer cancel()

	// Run daemon or one-shot mode
	if *daemon {
		logger.Info("Starting FaceLock daemon...")
		if err := runDaemon(ctx, cfg, logger); err != nil {
			logger.Fatalf("Daemon error: %v", err)
		}
	} else {
		logger.Info("Running FaceLock in one-shot mode")
		if err := runOneShot(ctx, cfg, logger); err != nil {
			logger.Fatalf("One-shot error: %v", err)
		}
	}
}

func loadConfiguration(path string, logger *logrus.Logger) *config.Config {
	cfg, err := config.Load(path)
	if err != nil {
		logger.Warnf("Failed to load config from %s: %v", path, err)
		logger.Info("Using default configuration")
		cfg = config.DefaultConfig()
	}
	return cfg
}

func setupSignalHandling(logger *logrus.Logger, configPath string, cfg **config.Config) (context.Context, context.CancelFunc) {
	ctx, cancel := context.WithCancel(context.Background())

	sigChan := make(chan os.Signal, 1)
	signal.Notify(sigChan, syscall.SIGINT, syscall.SIGTERM, syscall.SIGHUP)

	go func() {
		for sig := range sigChan {
			switch sig {
			case syscall.SIGINT, syscall.SIGTERM:
				logger.Info("Received shutdown signal")
				cancel()
			case syscall.SIGHUP:
				logger.Info("Received reload signal (SIGHUP)")
				// Reload configuration
				newCfg, err := config.Load(configPath)
				if err != nil {
					logger.Errorf("Failed to reload config: %v", err)
				} else if err := newCfg.Validate(); err != nil {
					logger.Errorf("Invalid configuration on reload: %v", err)
				} else {
					*cfg = newCfg
					logger.Info("Configuration reloaded successfully")
				}
			}
		}
	}()

	return ctx, cancel
}

func runDaemon(ctx context.Context, cfg *config.Config, logger *logrus.Logger) error {
	logger.Info("Starting FaceLock daemon...")

	// Initialize authentication engine
	engine, err := auth.NewEngine(cfg, logger)
	if err != nil {
		return fmt.Errorf("failed to create auth engine: %w", err)
	}
	defer engine.Close()

	// Create Unix socket for IPC
	socketPath := "/var/run/facelock/facelock.sock"
	if err := os.MkdirAll("/var/run/facelock", 0755); err != nil {
		logger.Warnf("Failed to create socket directory: %v", err)
		socketPath = "/tmp/facelock.sock"
	}

	// Remove existing socket if it exists
	os.Remove(socketPath)

	listener, err := net.Listen("unix", socketPath)
	if err != nil {
		return fmt.Errorf("failed to create Unix socket: %w", err)
	}
	defer listener.Close()
	defer os.Remove(socketPath)

	// Set socket permissions
	if err := os.Chmod(socketPath, 0660); err != nil {
		logger.Warnf("Failed to set socket permissions: %v", err)
	}

	logger.Infof("Daemon listening on %s", socketPath)

	// Accept connections in a goroutine
	go func() {
		for {
			conn, err := listener.Accept()
			if err != nil {
				select {
				case <-ctx.Done():
					return
				default:
					logger.Errorf("Accept error: %v", err)
					continue
				}
			}

			// Handle connection in separate goroutine
			go handleConnection(conn, engine, logger)
		}
	}()

	// Wait for shutdown signal
	<-ctx.Done()
	logger.Info("Daemon shutting down...")

	return nil
}

func handleConnection(conn net.Conn, engine *auth.Engine, logger *logrus.Logger) {
	defer conn.Close()

	// Simple protocol: read username, perform auth, write result
	buf := make([]byte, 256)
	n, err := conn.Read(buf)
	if err != nil {
		logger.Errorf("Read error: %v", err)
		return
	}

	username := string(buf[:n])
	logger.Infof("Authentication request for user: %s", username)

	// Perform authentication
	authCtx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()

	result, err := engine.AuthenticateUser(authCtx, username)
	if err != nil {
		conn.Write([]byte("ERROR: " + err.Error()))
		return
	}

	if result.Success {
		conn.Write([]byte("SUCCESS"))
	} else {
		conn.Write([]byte("FAILED"))
	}
}

func runOneShot(ctx context.Context, cfg *config.Config, logger *logrus.Logger) error {
	// One-shot mode is primarily for testing
	// The actual authentication happens through PAM
	logger.Info("One-shot mode - use facelock-test for authentication testing")
	return nil
}

func printVersion() {
	fmt.Println("FaceLock - Linux Face Recognition System")
	fmt.Println("========================================")
	fmt.Println("Version: 0.1.0 (PoC)")
	fmt.Println("License: MIT")
	fmt.Println()
	fmt.Println("Features:")
	fmt.Println("  - SCRFD face detection")
	fmt.Println("  - ArcFace recognition")
	fmt.Println("  - Depth-based liveness detection")
	fmt.Println("  - Challenge-response authentication")
	fmt.Println("  - PAM integration")
	fmt.Println()
	fmt.Println("Hardware Support:")
	fmt.Println("  - V4L2 cameras (RGB)")
	fmt.Println("  - IR cameras")
	fmt.Println("  - Intel RealSense depth cameras")
}
