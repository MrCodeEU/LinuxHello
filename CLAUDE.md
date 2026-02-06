# CLAUDE.md

This file provides guidance to Claude Code (claude.ai/code) when working with code in this repository.

## Project Overview

LinuxHello is an experimental Windows Hello-style face authentication system for Linux. It provides PAM integration for sudo, login screens, and system unlock using IR camera-based face recognition.

**Warning**: This is experimental software tested primarily on Fedora 41 + KDE Plasma 6.

## Architecture

The system consists of three main components:

1. **linuxhello** (Go/Wails v2) - Single binary with subcommands: GUI (default), daemon, enroll, test
2. **pam_linuxhello.so** (Go/CGO) - PAM module for system authentication integration
3. **inference_service.py** (Python) - gRPC AI service on port 50051 for face detection/recognition using ONNX models

```
┌─────────────────┐     ┌──────────────────┐     ┌─────────────────────┐
│  PAM Module     │────▶│  linuxhello      │────▶│  inference_service  │
│  (C shared obj) │     │  (Go daemon)     │     │  (Python gRPC)      │
└─────────────────┘     └──────────────────┘     └─────────────────────┘
                               │                          │
                        ┌──────┴──────┐           ┌───────┴───────┐
                        │   Camera    │           │  ONNX Models  │
                        │  (go4vl)    │           │  (SCRFD/Arc)  │
                        └─────────────┘           └───────────────┘
```

## Key Directories

- `internal/` - Core packages: auth (engine, liveness, lockout, challenge), camera, config, embedding (SQLite store), cli, daemon
- `pkg/pam/` - PAM module (CGO, builds as shared library)
- `python-service/` - Python inference service with ONNX models
- `frontend/` - React 19 + TypeScript + Vite frontend (embedded in Wails GUI binary)
- `frontend/wailsjs/` - Auto-generated Wails Go bindings and runtime stubs
- `app.go` + `main.go` - Wails v2 desktop app (GUI binary, builds from project root)
- `api/inference/` - Protobuf definitions for Go-Python gRPC communication

## Build Commands

```bash
# Complete setup (Python venv + Go deps + download AI models)
make setup

# Build all binaries (linuxhello + pam_linuxhello.so)
make build

# Build individual components
make build-app      # linuxhello (Wails app with all subcommands, includes frontend build)
make build-pam      # pam_linuxhello.so (CGO)
```

## Testing

```bash
# Run Go unit tests
make test
# Or directly:
go test -v ./...

# Run a specific test
go test -v ./internal/embedding -run TestStoreName

# Integration testing (requires camera)
make test-enroll    # Test face enrollment
make test-auth      # Test authentication
```

## Code Quality

```bash
make fmt    # Format Go code
make vet    # Run go vet
make lint   # Format + vet + golangci-lint (if installed)
```

## Running for Development

```bash
# Start inference service in background
make start-service-bg

# Stop inference service
make stop-service

# Run desktop GUI (starts inference service + launches Wails app)
make gui

# Check service status
make status
```

## Configuration

Default config: `/etc/linuxhello/linuxhello.conf`
Dev config: `configs/dev/linuxhello.conf` (created by test targets)

Key settings:
- `camera.device` - Video device path (e.g., /dev/video0)
- `recognition.threshold` - Face similarity threshold (0.5-0.8)
- `auth.timeout` - Authentication timeout in seconds

## Technology Stack

**Go 1.24+**: logrus (logging), viper (config), gRPC, go4vl (V4L2), sqlite3, Wails v2 (desktop GUI)
**Python**: ONNX Runtime, numpy, opencv, gRPC
**Frontend**: React 19, TypeScript, Vite, Tailwind CSS (embedded in Wails binary via `//go:embed`)

## gRPC Protocol

The Go daemon communicates with Python inference service via gRPC (defined in `api/inference/inference.proto`). The Python service handles face detection (SCRFD model) and recognition (ArcFace model).

## PAM Integration

PAM module is managed via `scripts/linuxhello-pam` script:
```bash
make pam-enable-sudo    # Enable for sudo
make pam-disable-all    # Disable all
make pam-restore        # Restore from backup
```

## Packaging

```bash
make package            # Build RPM, DEB, tarball
make build-rpm          # Build RPM only
make set-version VERSION=X.Y.Z  # Update version across files
```
