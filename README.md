# LinuxHello - Face Authentication for Linux

> ⚠️ **EXPERIMENTAL PROJECT - USE AT YOUR OWN RISK**
>
> This project is in early development and has **NOT been thoroughly tested**. It may contain bugs, security vulnerabilities, or cause system lockouts. **Always keep alternative authentication methods available.**
>
> **Tested Platform**: Fedora 41 + KDE Plasma 6 only. Other distributions may work but are untested.

A Windows Hello-style face authentication system for Linux. Uses IR cameras for secure face recognition with anti-spoofing protection.

## ⚠️ Prerequisites

### Required: Linux IR Emitter Support

Before using LinuxHello, you **MUST** set up IR camera support using the `linux-enable-ir-emitter` project:

```bash
# Install linux-enable-ir-emitter
# See: https://github.com/EmixamPP/linux-enable-ir-emitter

# Fedora
sudo dnf copr enable emixampp/linux-enable-ir-emitter
sudo dnf install linux-enable-ir-emitter

# Ubuntu/Debian
# Follow instructions at the GitHub repository

# Configure your IR camera
sudo linux-enable-ir-emitter configure

# Test that IR emitter works
linux-enable-ir-emitter run
```

**Without proper IR emitter configuration, face detection will fail in low-light conditions.**

### Hardware Requirements

- **IR Camera**: Windows Hello compatible camera (most modern laptops have these)
- **Supported Cameras**: Any V4L2-compatible IR camera
- **Tested Hardware**: 
  - Laptop integrated IR cameras (Lenovo ThinkPad, Dell XPS, HP EliteBook)
  - Intel RealSense D400 series

### Software Requirements

- Linux (Fedora 39+, Ubuntu 22.04+, Debian 12+, Arch Linux)
- Go 1.21+
- Python 3.10-3.12 (for inference service)
- GCC/G++ compiler

## Features

- **IR Face Recognition**: Works in complete darkness using IR cameras
- **Anti-Spoofing**: Rejects photos/screens (IR-based detection)
- **Fast Authentication**: <500ms typical authentication time
- **Challenge-Response**: Optional head movement verification
- **PAM Integration**: Works with sudo, login, screen lock
- **Auto-Lock**: Automatic screen lock when user leaves (planned)

## Quick Start

### 1. Clone and Build

```bash
git clone https://github.com/yourusername/LinuxHello.git
cd LinuxHello

# Setup Python environment and dependencies
make setup

# Build all binaries
make build
```

### 2. Download AI Models

```bash
# Download required ONNX models
make download-models

# Or manually download:
# - SCRFD face detection: models/scrfd_person_2.5g.onnx
# - ArcFace recognition: models/arcface_r50.onnx
```

### 3. Enroll Your Face

```bash
# Start enrollment (captures 5 samples)
make test-enroll

# Or run directly with debug output
./bin/facelock-enroll -user $USER -debug
```

### 4. Test Authentication

```bash
# Test face authentication
make test-auth

# Or run directly
./bin/facelock-test
```

## Platform Support

| Platform | Status | Notes |
|----------|--------|-------|
| **Fedora + KDE** | ✅ Tested | Primary development platform |
| Fedora + GNOME | 🔶 Untested | Should work |
| Ubuntu + KDE | 🔶 Untested | Should work |
| Ubuntu + GNOME | 🔶 Untested | Should work |
| Arch Linux | 🔶 Untested | Should work |
| Debian | 🔶 Untested | Should work |
| Other | ❓ Unknown | May require modifications |

## PAM Integration

> ⚠️ **WARNING**: Misconfiguring PAM can lock you out of your system!  
> Always keep a root terminal open when testing PAM changes.

### Automated PAM Management (Recommended)

LinuxHello includes a safe PAM management tool with automatic backups:

```bash
# Check current PAM status
facelock-pam status

# Test face authentication before enabling
facelock-pam test

# Enable for sudo (safest first step)
sudo facelock-pam enable sudo

# Enable for GUI password dialogs (PolicyKit)
sudo facelock-pam enable polkit

# Enable for SDDM login (KDE)
sudo facelock-pam enable sddm

# Disable if something goes wrong
sudo facelock-pam disable sudo
sudo facelock-pam disable polkit

# Restore original configs from backup
sudo facelock-pam restore

# List all available services
facelock-pam list
```

Convenience make targets:

```bash
make pam-status        # Show PAM integration status
make pam-enable-sudo   # Enable for sudo
make pam-enable-polkit # Enable for GUI dialogs
make pam-enable-sddm   # Enable for SDDM
make pam-disable-all   # Disable everything
make pam-restore       # Restore all backups
```

### Supported Services

| Service | Description | Risk Level |
|---------|-------------|------------|
| `sudo` | Terminal sudo commands | 🟢 Low (fallback to password) |
| `su` | Switch user command | 🟢 Low (fallback to password) |
| `polkit` | GUI password dialogs | 🟢 Low (fallback to password) |
| `sddm` | KDE display manager | 🟡 Medium |
| `gdm` | GNOME display manager | 🟡 Medium |
| `lightdm` | LightDM display manager | 🟡 Medium |
| `login` | TTY console login | 🟡 Medium |
| `system-auth` | Fedora/RHEL global auth | 🔴 High (affects everything) |
| `common-auth` | Debian/Ubuntu global auth | 🔴 High (affects everything) |
| `kde` | KDE screen locker | 🟡 Medium |

### Safety Features

- **Automatic Backups**: All original configs saved to `/var/lib/facelock/pam-backups/`
- **Password Fallback**: Face auth always falls back to password on failure
- **Lockout Recovery**: Boot to single-user mode and run `facelock-pam restore`
- **Timestamped Backups**: Multiple backup versions preserved
- **Dry-Run Support**: Use `-n` flag to see changes without applying

### Emergency Recovery

If you get locked out:

1. **Reboot** and enter GRUB menu
2. **Edit boot entry**: Add `init=/bin/bash` to kernel line
3. **Remount root**: `mount -o remount,rw /`
4. **Restore PAM**: `/usr/local/bin/facelock-pam restore`
5. **Reboot**: `reboot -f`

Or from a live USB:

```bash
# Mount your root partition
sudo mount /dev/sdXn /mnt

# Restore PAM backups
sudo cp /mnt/var/lib/facelock/pam-backups/*.backup /mnt/etc/pam.d/
```

## Configuration

Configuration file: `/etc/facelock/facelock.conf` or `~/.config/facelock/facelock.conf`

```yaml
camera:
  device: "/dev/video0"  # IR camera device
  width: 640
  height: 480

recognition:
  threshold: 0.6         # Similarity threshold (0.5-0.8)
  
liveness:
  enabled: true          # Anti-spoofing check
  
auth:
  timeout: 10            # Seconds before timeout
  max_attempts: 3        # Max failed attempts
```

## Project Structure

```
LinuxHello/
├── cmd/                    # Command-line tools
│   ├── facelock/          # Main PAM daemon
│   ├── facelock-enroll/   # Face enrollment tool
│   └── facelock-test/     # Authentication tester
├── internal/              # Core Go packages
│   ├── auth/              # Authentication engine
│   ├── camera/            # V4L2 camera interface
│   ├── config/            # Configuration loader
│   └── embedding/         # Face embedding storage
├── pkg/                   # Public packages
│   ├── models/            # gRPC client
│   └── pam/               # PAM module
├── python-service/        # Python inference service
│   ├── inference_service.py
│   └── requirements.txt
├── models/                # ONNX models (download separately)
├── configs/               # Configuration templates
└── scripts/               # Installation scripts
```

## Development

### Build Commands

```bash
make build          # Build all binaries
make test           # Run tests
make lint           # Run linter
make clean          # Clean build artifacts
make setup          # Setup Python environment
```

### Testing

```bash
make test-enroll    # Test enrollment process
make test-auth      # Test authentication
```

## Troubleshooting

### "No face detected"

1. Ensure IR emitter is working: `linux-enable-ir-emitter run`
2. Check camera device: `v4l2-ctl --list-devices`
3. Try in better lighting first to verify camera works

### "IR camera not found"

```bash
# List all video devices
ls -la /dev/video*

# Check camera info
v4l2-ctl -d /dev/video0 --all

# Your IR camera might be /dev/video2 or similar
```

### Authentication fails with enrolled face

1. Re-enroll with better lighting: `make test-enroll`
2. Lower the similarity threshold in config
3. Check logs: `cat logs/inference.log`

### PAM lockout recovery

```bash
# Boot into recovery mode or use TTY
# Remove the PAM line you added:
sudo nano /etc/pam.d/sudo
# Delete the "auth sufficient pam_facelock.so" line
```

## Roadmap

- [x] Basic face detection and recognition
- [x] IR camera support
- [x] Anti-spoofing (IR-based)
- [x] Enrollment tool
- [ ] GUI Settings application
- [ ] Auto-lock when user leaves
- [ ] Challenge-response UI
- [ ] SDDM/GDM integration guides
- [ ] Debian/Ubuntu packages
- [ ] Fedora COPR package

## Security Considerations

⚠️ **This is NOT a security-hardened implementation.**

- Face recognition can potentially be fooled
- Always use as a convenience feature, not sole authentication
- Keep password/PIN as backup
- Do not use for high-security systems

## License

MIT License - See [LICENSE](LICENSE) for details.

## Acknowledgments

- [InsightFace](https://github.com/deepinsight/insightface) - SCRFD and ArcFace models
- [linux-enable-ir-emitter](https://github.com/EmixamPP/linux-enable-ir-emitter) - IR camera support
- [go4vl](https://github.com/vladimirvivien/go4vl) - V4L2 Go bindings
- [ONNX Runtime](https://onnxruntime.ai/) - Model inference

## Contributing

Contributions welcome! Please open an issue first to discuss changes.

---

**⚠️ Remember: This project is experimental. Always maintain backup authentication methods.**
