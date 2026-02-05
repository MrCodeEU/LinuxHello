# LinuxHello - Face Authentication for Linux

> ⚠️ **EXPERIMENTAL PROJECT - USE AT YOUR OWN RISK**
>
> This project is in early development and has **NOT been thoroughly tested**. It may contain bugs, security vulnerabilities, or cause system lockouts. **Always keep alternative authentication methods available.**
>
> **Tested Platform**: Fedora 41 + KDE Plasma 6 only. Other distributions may work but are untested.

A Windows Hello-style face authentication system for Linux that provides:

- 🔐 **PAM Integration**: Face authentication for `sudo`, login screens, and system unlock
- 🌐 **Web Management**: Easy enrollment and configuration through browser interface  
- 📷 **IR Camera Support**: Uses infrared cameras with anti-spoofing protection
- 🧠 **AI-Powered**: Real-time face detection and recognition using ONNX models
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
sudo systemctl enable --now linuxhello-inference linuxhello-gui
```

### Quick Setup  
```bash
# 1. Open web interface
firefox http://localhost:8080

# 2. Enroll your face in the GUI, then:
sudo linuxhello-pam enable sudo
sudo -k && sudo ls  # Test face authentication!
```

## 🎯 Features

- **🔐 PAM Integration**: Face authentication for `sudo`, login, and screen unlock
- **🌐 Web Interface**: Easy enrollment and management through browser  
- **📷 IR Camera Support**: Works in darkness with anti-spoofing protection
- **🧠 AI Recognition**: Fast, accurate face detection and recognition
- **👁️ Visual Debugging**: Real-time detection visualization and confidence scores
- **👥 Multi-User**: Support for multiple enrolled users

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

Default config at `/etc/linuxhello/linuxhello.conf`:
```yaml
camera:
  device: "/dev/video0"    # Your IR camera
recognition:
  threshold: 0.6           # Similarity threshold (0.5-0.8)
auth:
  timeout: 10             # Seconds before timeout
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
