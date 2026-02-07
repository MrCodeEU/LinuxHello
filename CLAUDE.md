# CLAUDE.md

This file provides guidance to Claude Code (claude.ai/code) when working with code in this repository.

## Project Overview

LinuxHello is an experimental Windows Hello-style face authentication system for Linux. It provides PAM integration for sudo, login screens, and system unlock using IR camera-based face recognition.

**Warning**: This is experimental software tested primarily on Fedora 41 + KDE Plasma 6.

## Architecture

The system consists of three main components:

1. **linuxhello GUI** (Go/Wails v2) - Desktop app with embedded React frontend, runs HTTP server on :8080
2. **pam_linuxhello.so** (Go/CGO) - PAM module for system authentication integration
3. **inference_service.py** (Python) - gRPC AI service on port 50051 for face detection/recognition using ONNX models

```
┌─────────────────┐     ┌──────────────────┐     ┌─────────────────────┐
│  PAM Module     │────▶│  linuxhello GUI │────▶│  inference_service  │
│  (C shared obj) │     │  (Wails+HTTP)   │     │  (Python gRPC)      │
└─────────────────┘     └──────────────────┘     └─────────────────────┘
                               │                          │
                        ┌──────┴──────┐           ┌───────┴───────┐
                        │   Camera    │           │  ONNX Models  │
                        │  (go4vl)    │           │  (SCRFD/Arc)  │
                        └─────────────┘           └───────────────┘
                               │
                        ┌──────┴──────────────┐
                        │ React Frontend     │
                        │ (localhost:8080)   │
                        └────────────────────┘
```

### GUI Architecture (v1.3.4+)
The `linuxhello` binary runs an HTTP server serving:
- **Frontend**: React SPA at `http://localhost:8080` (built from `frontend/`, served from embedded FS)
- **API Endpoints**: RESTful API for enrollment, auth testing, config, PAM management, logs
- **Camera Streaming**: MJPEG stream at `/api/stream` for live preview
- **Real-time Updates**: Enrollment progress polling, log viewing, service status

## Key Directories

- `cmd/linuxhello-gui/` - Main HTTP server (main.go) with REST API endpoints, camera streaming, enrollment logic
- `internal/` - Core packages: auth (engine, liveness, lockout, challenge), camera, config, embedding (SQLite store)
- `pkg/pam/` - PAM module (CGO, builds as shared library)
- `python-service/` - Python inference service with ONNX models
- `frontend/` - React 19 + TypeScript + Vite frontend (embedded in binary at compile time)
  - `src/components/` - UI components (EnrollmentTab, AuthTestTab, LogsTab, SettingsTab, etc.)
  - `src/hooks/` - React hooks for data fetching and state management
- `models/` - ONNX model files (downloaded by `make setup`)
- `scripts/` - Helper scripts (linuxhello-pam for PAM management)
- `systemd/` - systemd service files

## Build Commands

```bash
# Complete setup (Python venv + Go deps + download AI models + build frontend)
make setup

# Build all binaries (linuxhello-gui + pam_linuxhello.so + frontend)
make build

# Build individual components
make build-frontend # npm run build in frontend/
make build-gui      # linuxhello-gui HTTP server (includes frontend embed)
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

# Run GUI (starts both services and opens browser to localhost:8080)
sudo linuxhello
# Or via desktop launcher after installation

# Check service status
make status
sudo systemctl status linuxhello-inference
sudo systemctl status linuxhello-gui
```

## API Endpoints (localhost:8080)

The GUI HTTP server provides:
- `GET /api/stream` - MJPEG camera stream
- `GET /api/users` - List enrolled users
- `POST /api/enroll` - Start enrollment (username in body)
- `GET /api/enroll/status` - Poll enrollment progress (real-time)
- `GET/POST /api/config` - Get/update configuration
- `GET/POST /api/pam` - PAM status and management
- `GET /api/logs` - View systemd service logs (journalctl integration)
- `GET /api/logs/download` - Download full logs
- `POST /api/authtest` - Test authentication with current camera frame
- `POST /api/service` - Control systemd services (start/stop/restart)

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
