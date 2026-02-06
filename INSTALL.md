# LinuxHello Installation Guide

## Quick Install (RPM - Recommended for Fedora/RHEL/CentOS)

### 1. Download and Install
```bash
# Download the latest RPM package
wget https://github.com/MrCodeEU/LinuxHello/releases/latest/download/linuxhello-*.rpm

# Install the package
sudo dnf install linuxhello-*.rpm
# or for older systems:
# sudo yum install linuxhello-*.rpm
```

### 2. Download AI Models
```bash
# Download the model download helper
sudo wget https://github.com/MrCodeEU/LinuxHello/releases/latest/download/download-models.sh

# Make it executable and run
sudo chmod +x download-models.sh
sudo ./download-models.sh
```

### 3. Start Services
```bash
# Enable and start the inference service
sudo systemctl enable --now linuxhello-inference
```

### 4. Set Up Face Authentication
```bash
# Launch the desktop GUI
sudo linuxhello

# Or use the command line:
# 1. Enroll your face
sudo linuxhello enroll -user $USER

# 2. Enable PAM for sudo
sudo linuxhello-pam enable sudo

# 3. Test it
sudo -k && sudo ls  # Should authenticate with your face!
```

## Alternative Installation Methods

### Debian/Ubuntu (DEB Package)
```bash
# Download and install DEB
wget https://github.com/MrCodeEU/LinuxHello/releases/latest/download/linuxhello_*_amd64.deb
sudo apt install ./linuxhello_*.deb

# Continue with steps 2-4 above
```

### Generic Installation (Any Linux)
```bash
# Download tarball
wget https://github.com/MrCodeEU/LinuxHello/releases/latest/download/linuxhello-*-linux-*.tar.gz
tar -xzf linuxhello-*.tar.gz
cd linuxhello-generic

# Install
sudo ./install.sh

# Continue with steps 2-4 above
```

### Build from Source
```bash
# Clone repository
git clone https://github.com/MrCodeEU/LinuxHello.git
cd LinuxHello

# Install dependencies (Fedora)
sudo dnf install golang gcc libv4l-devel pam-devel sqlite-devel nodejs npm

# Build and install
make setup
make build
sudo make install

# Continue with steps 2-4 above
```

## Usage

### Desktop GUI
- Launch with: `sudo linuxhello`
- **Enroll Face**: Add your face to the system
- **Auth Test**: Test face detection and authentication with visual debugging
- **User Manager**: Manage enrolled users
- **System & PAM**: Enable/disable PAM integration
- **Configuration**: Adjust system settings

### Command Line Tools
- `linuxhello enroll -user USERNAME`: Enroll a user's face
- `linuxhello test -user USERNAME`: Test authentication
- `linuxhello-pam enable sudo`: Enable face auth for sudo
- `linuxhello-pam status`: Show PAM integration status

### Services
- `linuxhello-inference`: AI inference service (required)
- `linuxhello`: Desktop management application (launch with `sudo linuxhello`)

## Troubleshooting

### Camera Issues
- Ensure your camera is accessible: `ls /dev/video*`
- Check camera permissions for the linuxhello user
- Test with: `sudo linuxhello test`

### PAM Not Working
- Check logs: `journalctl -u linuxhello-inference`
- Verify PAM module: `ldd /usr/lib/security/pam_linuxhello.so`
- Test in GUI: launch `sudo linuxhello` → "Auth Test" tab

### Service Issues
```bash
# Check service status
sudo systemctl status linuxhello-inference

# View logs
sudo journalctl -u linuxhello-inference -f
```

### Model Issues
- Models should be in `/usr/share/linuxhello/models/`
- Required: `arcface_r50.onnx`, `scrfd_person_2.5g.onnx`
- Re-run model download script if needed

## Uninstallation

### RPM/DEB
```bash
# Fedora/RHEL
sudo dnf remove linuxhello

# Ubuntu/Debian  
sudo apt remove linuxhello
```

### Manual Cleanup
```bash
# Stop services
sudo systemctl stop linuxhello-inference
sudo systemctl disable linuxhello-inference

# Remove PAM integration
sudo linuxhello-pam disable sudo

# Remove files
sudo rm -rf /etc/linuxhello /usr/share/linuxhello /var/lib/linuxhello
sudo rm -f /usr/bin/linuxhello* /usr/lib/security/pam_linuxhello.so
sudo rm -f /etc/systemd/system/linuxhello-*.service
```