// Package daemon provides the background daemon functionality for LinuxHello
package daemon

import (
	"context"
	"flag"
	"fmt"
	"net"
	"os"
	"os/signal"
	"syscall"
	"time"

	"github.com/MrCodeEU/LinuxHello/internal/auth"
	"github.com/MrCodeEU/LinuxHello/internal/config"
	"github.com/sirupsen/logrus"
)

// Run starts the daemon with the given arguments
func Run(args []string) {
	fs := flag.NewFlagSet("daemon", flag.ExitOnError)
	configPath := fs.String("config", "/etc/linuxhello/linuxhello.conf", "Path to configuration file")
	verbose := fs.Bool("verbose", false, "Enable verbose logging")
	version := fs.Bool("version", false, "Show version information")
	_ = fs.Parse(args)

	if *version {
		printVersion()
		return
	}

	logger := logrus.New()
	if *verbose {
		logger.SetLevel(logrus.DebugLevel)
	} else {
		logger.SetLevel(logrus.InfoLevel)
	}

	cfg := loadConfiguration(*configPath, logger)

	if err := cfg.Validate(); err != nil {
		logger.Fatalf("Invalid configuration: %v", err)
	}

	ctx, cancel := setupSignalHandling(logger, *configPath, &cfg)
	defer cancel()

	logger.Info("Starting LinuxHello daemon...")
	if err := runDaemon(ctx, cfg, logger); err != nil {
		logger.Fatalf("Daemon error: %v", err)
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
	logger.Info("Starting LinuxHello daemon...")

	engine, err := auth.NewEngine(cfg, logger)
	if err != nil {
		return fmt.Errorf("failed to create auth engine: %w", err)
	}
	defer func() {
		if err := engine.Close(); err != nil {
			logger.Errorf("Failed to close engine: %v", err)
		}
	}()

	socketPath := "/var/run/linuxhello/linuxhello.sock"
	if err := os.MkdirAll("/var/run/linuxhello", 0755); err != nil {
		logger.Warnf("Failed to create socket directory: %v", err)
		socketPath = "/tmp/linuxhello.sock"
	}

	_ = os.Remove(socketPath)

	listener, err := net.Listen("unix", socketPath)
	if err != nil {
		return fmt.Errorf("failed to create Unix socket: %w", err)
	}
	defer func() {
		if err := listener.Close(); err != nil {
			logger.Errorf("Failed to close listener: %v", err)
		}
	}()
	defer func() { _ = os.Remove(socketPath) }()

	if err := os.Chmod(socketPath, 0660); err != nil {
		logger.Warnf("Failed to set socket permissions: %v", err)
	}

	logger.Infof("Daemon listening on %s", socketPath)

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

			go handleConnection(conn, engine, logger)
		}
	}()

	<-ctx.Done()
	logger.Info("Daemon shutting down...")

	return nil
}

func handleConnection(conn net.Conn, engine *auth.Engine, logger *logrus.Logger) {
	defer func() { _ = conn.Close() }()

	buf := make([]byte, 256)
	n, err := conn.Read(buf)
	if err != nil {
		logger.Errorf("Read error: %v", err)
		return
	}

	username := string(buf[:n])
	logger.Infof("Authentication request for user: %s", username)

	authCtx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()

	result, err := engine.AuthenticateUser(authCtx, username)
	if err != nil {
		_, _ = conn.Write([]byte("ERROR: " + err.Error()))
		return
	}

	if result.Success {
		_, _ = conn.Write([]byte("SUCCESS"))
	} else {
		_, _ = conn.Write([]byte("FAILED"))
	}
}

func printVersion() {
	fmt.Println("LinuxHello Daemon")
	fmt.Println("=================")
	fmt.Println("Version: 1.3.4")
	fmt.Println("License: MIT")
}
