# LinuxHello Project Context

## Overview
**LinuxHello** is an open-source face authentication system for Linux, designed to mimic Windows Hello functionality. It provides secure, hands-free authentication for `sudo`, login screens (GDM/SDDM), and other PAM-aware services.

### Key Features
*   **Face Authentication:** Uses IR cameras and deep learning for secure recognition.
*   **PAM Integration:** Seamlessly integrates with system authentication via a custom PAM module.
*   **Liveness Detection:** Prevents spoofing using depth/IR analysis.
*   **Web Management UI:** A modern React-based interface for user enrollment and configuration.
*   **Privacy-First:** All processing happens locally on the device.

## Architecture
The system relies on a hybrid architecture:
1.  **Go Daemon (`cmd/linuxhello`):** The central coordinator handling auth logic, config, and IPC.
2.  **Python Inference Service (`python-service`):** A gRPC server running ONNX models (SCRFD for detection, ArcFace for recognition) to perform the heavy lifting of computer vision.
3.  **PAM Module (`pkg/pam`):** A C-shared Go library (`pam_linuxhello.so`) that hooks into the Linux authentication stack.
4.  **Web UI (`web-ui`):** A React/Vite frontend communicating with the daemon for management tasks.

## Tech Stack
*   **Backend:** Go (1.24+), gRPC, SQLite (auth data), CGO (PAM).
*   **AI/ML:** Python 3, ONNX Runtime, NumPy, OpenCV.
*   **Frontend:** React 19, TypeScript, Vite, Tailwind CSS.
*   **Build System:** Make, Shell scripts.

## Development Guide

### Prerequisites
*   **System:** Linux (Fedora/RHEL/CentOS recommended, others supported).
*   **Tools:** Go 1.24+, Python 3.9+, Node.js 18+, Make, GCC.
*   **Hardware:** IR-capable webcam (recommended) or standard webcam.

### Quick Start
The `Makefile` is the primary entry point for all development tasks.

```bash
# 1. Full Setup (Dependencies, Models, Python Venv)
make setup

# 2. Build All Components (Daemon, GUI, PAM, CLIs)
make build

# 3. Start Inference Service (Required for any operation)
make start-service-bg
```

### Key Commands
| Command | Description |
| :--- | :--- |
| `make build` | Compiles all Go binaries and the Web UI. |
| `make test-enroll` | Runs the CLI enrollment tool for testing. |
| `make test-auth` | Runs the CLI authentication test. |
| `make gui` | Builds and launches the Web Management Interface. |
| `make clean` | Removes build artifacts. |
| `make lint` | Runs Go linters (fmt, vet, golangci-lint). |

### Configuration
Configuration is managed via `configs/linuxhello.conf` (local dev) or `/etc/linuxhello/linuxhello.conf` (production).
*   **Key Settings:** Camera device path (`/dev/video0`), authentication thresholds, and liveness checks.
*   **Struct:** Defined in `internal/config/config.go`.

### Project Structure
*   `cmd/` - Entry points for binaries (`linuxhello`, `linuxhello-enroll`, `linuxhello-gui`).
*   `internal/` - Private application code (auth engine, camera handling, config).
*   `pkg/` - Library code (PAM module, shared utilities).
*   `python-service/` - ML inference engine (Python/ONNX).
*   `web-ui/` - React frontend source.
*   `models/` - Downloaded ONNX models (ArcFace, SCRFD).
*   `scripts/` - Helper scripts for PAM management and installation.

## Development Conventions
*   **Go:** Follows standard Go idioms. Use `make fmt` and `make vet` before committing.
*   **Python:** Dependencies in `python-service/requirements.txt`. Use the venv created by `make setup`.
*   **Frontend:** Modern React hooks pattern. Build with `npm run build` inside `web-ui`.
*   **Safety:** The project interacts with system security (PAM). Always test changes with `make test-auth` before enabling PAM integration system-wide to avoid lockouts.
