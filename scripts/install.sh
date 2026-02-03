#!/bin/bash
# FaceLock Installation Script
# Supports Fedora, RHEL, Ubuntu, Debian

set -e

# Colors for output
RED='\033[0;31m'
GREEN='\033[0;32m'
YELLOW='\033[1;33m'
NC='\033[0m' # No Color

# Script directory
SCRIPT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
PROJECT_ROOT="$(dirname "$SCRIPT_DIR")"

# Installation paths
PREFIX="/usr/local"
BINDIR="$PREFIX/bin"
LIBDIR="$PREFIX/lib"
SYSCONFDIR="/etc"
DATADIR="/usr/share"
PAMDIR="$LIBDIR/security"

# Detect distribution
detect_distro() {
    if [ -f /etc/os-release ]; then
        . /etc/os-release
        echo "$ID"
    elif [ -f /etc/redhat-release ]; then
        echo "fedora"
    elif [ -f /etc/debian_version ]; then
        echo "debian"
    else
        echo "unknown"
    fi
}

DISTRO=$(detect_distro)

echo -e "${GREEN}FaceLock Installation Script${NC}"
echo "=============================="
echo "Detected distribution: $DISTRO"
echo ""

# Check if running as root
check_root() {
    if [ "$EUID" -ne 0 ]; then
        echo -e "${RED}Error: This script must be run as root${NC}"
        echo "Please run with sudo: sudo $0"
        exit 1
    fi
}

# Install dependencies
install_dependencies() {
    echo -e "${YELLOW}Installing dependencies...${NC}"
    
    case "$DISTRO" in
        fedora|rhel|centos)
            dnf install -y \
                golang \
                gcc \
                gcc-c++ \
                make \
                libv4l-devel \
                onnxruntime-devel \
                pam-devel \
                sqlite-devel \
                librealsense2-devel 2>/dev/null || true
            ;;
        ubuntu|debian)
            apt-get update
            apt-get install -y \
                golang-go \
                gcc \
                g++ \
                make \
                libv4l-dev \
                libonnxruntime-dev 2>/dev/null || true \
                libpam0g-dev \
                libsqlite3-dev \
                librealsense2-dev 2>/dev/null || true
            ;;
        arch)
            pacman -S --needed \
                go \
                gcc \
                make \
                v4l-utils \
                onnxruntime 2>/dev/null || true \
                pam \
                sqlite \
                librealsense 2>/dev/null || true
            ;;
        *)
            echo -e "${YELLOW}Warning: Unknown distribution. Please install dependencies manually.${NC}"
            echo "Required packages: golang, gcc, make, libv4l-dev, onnxruntime-dev, pam-dev, sqlite-dev"
            ;;
    esac
    
    echo -e "${GREEN}Dependencies installed.${NC}"
}

# Download ONNX models
download_models() {
    echo -e "${YELLOW}Downloading ONNX models...${NC}"
    
    MODEL_DIR="$DATADIR/facelock/models"
    mkdir -p "$MODEL_DIR"
    
    # Model URLs (these would be actual download URLs)
    # For now, we'll create placeholder instructions
    
    cat > "$MODEL_DIR/README.txt" << 'EOF'
FaceLock ONNX Models
====================

Please download the following models and place them in this directory:

1. SCRFD 2.5G (Face Detection)
   - File: scrfd_person_2.5g.onnx
   - Source: https://github.com/deepinsight/insightface
   - Size: ~3-5 MB

2. ArcFace r50 (Face Recognition)
   - File: arcface_r50.onnx
   - Source: https://huggingface.co/onnxmodelzoo
   - Size: ~166 MB

3. Depth Liveness (Optional - for depth-based liveness)
   - File: depth_liveness.onnx
   - Source: Train custom or use pre-trained
   - Size: ~1-5 MB

Model Conversion:
-----------------
If you have PyTorch models, you can convert them to ONNX:

  import torch
  import torch.onnx
  
  # Load your model
  model = torch.load('model.pt')
  model.eval()
  
  # Create dummy input
  dummy_input = torch.randn(1, 3, 640, 640)
  
  # Export to ONNX
  torch.onnx.export(model, dummy_input, 'model.onnx',
                    input_names=['input'],
                    output_names=['output'],
                    dynamic_axes={'input': {0: 'batch_size'},
                                  'output': {0: 'batch_size'}})
EOF

    echo -e "${YELLOW}Note: Please download ONNX models manually.${NC}"
    echo "See $MODEL_DIR/README.txt for instructions."
}

# Build the project
build_project() {
    echo -e "${YELLOW}Building FaceLock...${NC}"
    
    cd "$PROJECT_ROOT"
    
    # Download Go dependencies
    go mod download
    
    # Build binaries
    go build -o bin/facelock ./cmd/facelock
    go build -o bin/facelock-enroll ./cmd/facelock-enroll
    go build -o bin/facelock-test ./cmd/facelock-test
    
    # Build PAM module (requires CGO)
    CGO_CFLAGS="-I/usr/include" \
    CGO_LDFLAGS="-lpam -lpam_misc" \
    go build -buildmode=c-shared -o bin/pam_facelock.so ./pkg/pam
    
    echo -e "${GREEN}Build complete.${NC}"
}

# Install files
install_files() {
    echo -e "${YELLOW}Installing FaceLock...${NC}"
    
    # Create directories
    mkdir -p "$BINDIR"
    mkdir -p "$PAMDIR"
    mkdir -p "$SYSCONFDIR/facelock"
    mkdir -p "$DATADIR/facelock/models"
    mkdir -p "/var/lib/facelock"
    mkdir -p "/var/log"
    
    # Install binaries
    install -m 755 "$PROJECT_ROOT/bin/facelock" "$BINDIR/"
    install -m 755 "$PROJECT_ROOT/bin/facelock-enroll" "$BINDIR/"
    install -m 755 "$PROJECT_ROOT/bin/facelock-test" "$BINDIR/"
    
    # Install PAM module
    install -m 755 "$PROJECT_ROOT/bin/pam_facelock.so" "$PAMDIR/"
    
    # Install configuration
    if [ ! -f "$SYSCONFDIR/facelock/facelock.conf" ]; then
        install -m 644 "$PROJECT_ROOT/configs/facelock.conf" "$SYSCONFDIR/facelock/"
    else
        echo -e "${YELLOW}Configuration file already exists, not overwriting.${NC}"
    fi
    
    # Install PAM configuration example
    install -m 644 "$PROJECT_ROOT/configs/pam/common-auth-facelock" "$SYSCONFDIR/facelock/"
    
    # Set permissions
    chmod 755 "/var/lib/facelock"
    
    # Create log file if it doesn't exist
    touch "/var/log/facelock.log"
    chmod 644 "/var/log/facelock.log"
    
    echo -e "${GREEN}Installation complete.${NC}"
}

# Setup user permissions
setup_permissions() {
    echo -e "${YELLOW}Setting up permissions...${NC}"
    
    # Add video group for camera access
    echo "To use FaceLock, users need to be in the 'video' group."
    echo "Run: sudo usermod -a -G video <username>"
    echo ""
    
    # Set capabilities for PAM module (optional, for better performance)
    if command -v setcap &> /dev/null; then
        setcap cap_sys_admin+ep "$BINDIR/facelock" 2>/dev/null || true
    fi
}

# Print post-installation instructions
print_instructions() {
    echo ""
    echo -e "${GREEN}FaceLock Installation Complete!${NC}"
    echo "================================"
    echo ""
    echo "Next steps:"
    echo ""
    echo "1. Download ONNX models:"
    echo "   cat $DATADIR/facelock/models/README.txt"
    echo ""
    echo "2. Add your user to the video group:"
    echo "   sudo usermod -a -G video \$USER"
    echo "   (Log out and back in for changes to take effect)"
    echo ""
    echo "3. Enroll your face:"
    echo "   facelock-enroll -user \$USER"
    echo ""
    echo "4. Test authentication:"
    echo "   facelock-test"
    echo ""
    echo "5. Enable PAM integration (optional):"
    echo "   Edit /etc/pam.d/common-auth or /etc/pam.d/system-auth"
    echo "   Add: auth sufficient pam_facelock.so"
    echo "   See: $SYSCONFDIR/facelock/common-auth-facelock"
    echo ""
    echo "6. Configuration file:"
    echo "   $SYSCONFDIR/facelock/facelock.conf"
    echo ""
    echo -e "${YELLOW}Warning: Test thoroughly before enabling PAM integration!${NC}"
    echo "A misconfigured PAM can lock you out of your system."
    echo ""
    echo "For help: facelock-enroll -help"
    echo "For issues: https://github.com/facelock/facelock/issues"
}

# Uninstall function
uninstall() {
    echo -e "${YELLOW}Uninstalling FaceLock...${NC}"
    
    rm -f "$BINDIR/facelock"
    rm -f "$BINDIR/facelock-enroll"
    rm -f "$BINDIR/facelock-test"
    rm -f "$PAMDIR/pam_facelock.so"
    
    echo -e "${YELLOW}Note: Configuration files and data were not removed.${NC}"
    echo "To remove all data, run:"
    echo "  sudo rm -rf $SYSCONFDIR/facelock"
    echo "  sudo rm -rf /var/lib/facelock"
    echo "  sudo rm -rf $DATADIR/facelock"
    
    echo -e "${GREEN}Uninstall complete.${NC}"
}

# Main installation flow
main() {
    # Parse arguments
    case "${1:-}" in
        --uninstall|-u)
            check_root
            uninstall
            exit 0
            ;;
        --help|-h)
            echo "FaceLock Installation Script"
            echo ""
            echo "Usage: $0 [OPTIONS]"
            echo ""
            echo "Options:"
            echo "  --help, -h       Show this help message"
            echo "  --uninstall, -u  Uninstall FaceLock"
            echo ""
            echo "Examples:"
            echo "  sudo $0          Install FaceLock"
            echo "  sudo $0 -u       Uninstall FaceLock"
            exit 0
            ;;
    esac
    
    check_root
    install_dependencies
    download_models
    build_project
    install_files
    setup_permissions
    print_instructions
}

main "$@"
