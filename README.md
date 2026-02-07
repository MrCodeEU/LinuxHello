# LinuxHello - Face Authentication for Linux

> ⚠️ **EXPERIMENTAL PROJECT - USE AT YOUR OWN RISK**
>
> This project is in early development and has **NOT been thoroughly tested**. It may contain bugs, security vulnerabilities, or cause system lockouts. **Always keep alternative authentication methods available.**
>
> **Tested Platform**: Fedora 41 + KDE Plasma 6 only. Other distributions may work but are untested.

A Windows Hello-style face authentication system for Linux that provides:

- 🔐 **PAM Integration**: Face authentication for `sudo`, login screens, and system unlock
- 🌐 **Web Management GUI**: Modern Wails-based desktop app with browser interface at `localhost:8080`
- 📷 **IR Camera Support**: Uses infrared cameras with anti-spoofing protection
- 🧠 **AI-Powered**: Real-time face detection and recognition using ONNX models
- 📊 **Real-time Monitoring**: Live enrollment progress, system logs, and authentication testing
- 🐧 **Linux Native**: Built specifically for Linux with systemd integration

## 🚀 Quick Start

### Installation

📦 **Easy Install** - Download pre-built packages:
- **[Complete Installation Guide](INSTALL.md)** - Step-by-step instructions
- **[Latest Releases](https://github.com/MrCodeEU/LinuxHello/releases)** - RPM, DEB, and generic packages

### Quick RPM Install (Fedora/RHEL/CentOS)
```bash
wget https://github.com/MrCodeEU/LinuxHello/releases/latest/download/linuxhello-*.rpm
sudo dnf install linuxhello-*.rpm
sudo systemctl enable --now linuxhello-inference
```

### Quick Setup
```bash
# 1. Launch the desktop GUI (automatically opens in browser)
sudo linuxhello
# Or use the desktop launcher: Applications → LinuxHello Face Authentication

# 2. In the web interface (localhost:8080):
#    - Go to "Enrollment" tab and enroll your face
#    - Watch real-time progress as it captures samples
#    - Test authentication in the "Auth Test" tab

# 3. Enable PAM authentication:
sudo linuxhello-pam enable sudo
sudo -k && sudo ls  # Test face authentication!
```

## 🎯 Features

### Core Authentication
- **🔐 PAM Integration**: Face authentication for `sudo`, login, and screen unlock
- **📷 IR Camera Support**: Works in darkness with anti-spoofing protection
- **🧠 AI Recognition**: Fast, accurate face detection and recognition using ArcFace
- **👥 Multi-User**: Support for multiple enrolled users with individual profiles

### Modern Web UI (Wails v2)
- **🌐 Desktop App**: Native desktop application with embedded web interface
- **📊 Real-time Enrollment**: Live progress bar showing sample capture (e.g., "Sample 3/5")
- **👁️ Live Camera Preview**: See exactly what the camera sees during enrollment
- **🧪 Authentication Testing**: Test face recognition with bounding box visualization
- **📋 System Logs**: View and download systemd service logs with filtering
- **⚙️ Configuration**: Adjust thresholds, camera settings, and authentication parameters
- **🎮 Service Control**: Start/stop/restart inference and GUI services
- **🔧 PAM Management**: One-click enable/disable PAM authentication

## ⚠️ Hardware Requirements

### IR Camera + linux-enable-ir-emitter

**Required Setup** (do this first):
```bash
# Install IR emitter support
sudo dnf copr enable emixampp/linux-enable-ir-emitter
sudo dnf install linux-enable-ir-emitter

# Configure your camera
sudo linux-enable-ir-emitter configure
```

**Supported Hardware**: Windows Hello compatible IR cameras (most laptops have these)

## 🏗️ Development

### From Source
```bash
git clone https://github.com/MrCodeEU/LinuxHello.git
cd LinuxHello
make setup && make build
```

### Local Testing with Act
```bash
# Test GitHub workflow locally
./test-workflow.sh
```

## Platform Support

| Platform | Status | Notes |
|----------|--------|-------|
| **Fedora + KDE** | ✅ Tested | Primary development platform |
| Fedora + GNOME | 🔶 Untested | Should work |
| Ubuntu + KDE | 🔶 Untested | Should work |
| Ubuntu + GNOME | 🔶 Untested | Should work |
## 🛠️ Configuration

### Web Interface Configuration
Access the GUI at `http://localhost:8080` or launch via:
- Command: `sudo linuxhello`
- Desktop: Applications → LinuxHello Face Authentication

The **Settings** tab provides:
- Camera device selection and resolution
- Detection confidence and NMS thresholds  
- Recognition similarity threshold (0.5-0.8)
- Enrollment sample count
- Logging level configuration

Changes are saved to `/etc/linuxhello/linuxhello.conf` (or `/var/lib/linuxhello/` if permissions restrict).

### Manual Configuration
Edit `/etc/linuxhello/linuxhello.conf`:
```yaml
camera:
  device: "/dev/video0"    # Your IR camera
  width: 1280              # Camera resolution
  height: 720
  
recognition:
  similarity_threshold: 0.6   # Match threshold (0.5-0.8)
  enrollment_samples: 5       # Samples to capture
  
auth:
  timeout: 10                 # Seconds before timeout
```

## ⚠️ Safety & Recovery

**PAM Safety Features:**
- Automatic backups of all PAM configs
- Always falls back to password on face auth failure  
- Emergency recovery: Boot to single-user and run `linuxhello-pam restore`

**Keep Alternative Access:**
- Always test with `sudo -k && sudo ls` first
- Keep a root terminal open when testing PAM changes
- Have a live USB ready for emergency recovery

## 🤝 Contributing & Support

- **Documentation**: See [INSTALL.md](INSTALL.md) for detailed setup
- **Issues**: Report bugs and feature requests on GitHub
- **Development**: `make setup && make build` to build from source

## 📝 License

MIT License - See LICENSE file for details

---

**⚠️ Disclaimer**: This is experimental software. Use at your own risk and always maintain alternative authentication methods.

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
# Delete the "auth sufficient pam_linuxhello.so" line
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
