#!/bin/bash
# LinuxHello Installation Script
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

echo -e "${GREEN}LinuxHello Installation Script${NC}"
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
    
    MODEL_DIR="$DATADIR/linuxhello/models"
    mkdir -p "$MODEL_DIR"
    
    # Model URLs (these would be actual download URLs)
    # For now, we'll create placeholder instructions
    
    cat > "$MODEL_DIR/README.txt" << 'EOF'
LinuxHello ONNX Models
====================

Please download the following models and place them in this directory:

1. SCRFD 2.5G (Face Detection)
   - File: det_10g.onnx
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
    echo -e "${YELLOW}Building LinuxHello...${NC}"
    
    cd "$PROJECT_ROOT"
    
    # Download Go dependencies
    go mod download
    
    # Build frontend assets
    cd frontend && npm install && npm run build && cd ..

    # Build single binary (Wails app with all subcommands)
    go build -tags desktop,production -o bin/linuxhello .

    # Build PAM module (requires CGO)
    CGO_CFLAGS="-I/usr/include" \
    CGO_LDFLAGS="-lpam -lpam_misc" \
    go build -buildmode=c-shared -o bin/pam_linuxhello.so ./pkg/pam
    
    echo -e "${GREEN}Build complete.${NC}"
}

# Install files
install_files() {
    echo -e "${YELLOW}Installing LinuxHello...${NC}"
    
    # Create directories
    mkdir -p "$BINDIR"
    mkdir -p "$PAMDIR"
    mkdir -p "$SYSCONFDIR/linuxhello"
    mkdir -p "$DATADIR/linuxhello/models"
    mkdir -p "/var/lib/linuxhello"
    mkdir -p "/var/log"
    
    # Install binaries
    install -m 755 "$PROJECT_ROOT/bin/linuxhello" "$BINDIR/"
    
    # Install PAM module
    install -m 755 "$PROJECT_ROOT/bin/pam_linuxhello.so" "$PAMDIR/"
    
    # Install configuration
    if [ ! -f "$SYSCONFDIR/linuxhello/linuxhello.conf" ]; then
        install -m 644 "$PROJECT_ROOT/configs/linuxhello.conf" "$SYSCONFDIR/linuxhello/"
    else
        echo -e "${YELLOW}Configuration file already exists, not overwriting.${NC}"
    fi
    
    # Install PAM configuration example
    install -m 644 "$PROJECT_ROOT/configs/pam/common-auth-linuxhello" "$SYSCONFDIR/linuxhello/"
    
    # Set permissions
    chmod 755 "/var/lib/linuxhello"
    
    # Create log file if it doesn't exist
    touch "/var/log/linuxhello.log"
    chmod 644 "/var/log/linuxhello.log"
    
    echo -e "${GREEN}Installation complete.${NC}"
}

# Setup user permissions
setup_permissions() {
    echo -e "${YELLOW}Setting up permissions...${NC}"
    
    # Add video group for camera access
    echo "To use LinuxHello, users need to be in the 'video' group."
    echo "Run: sudo usermod -a -G video <username>"
    echo ""
    
    # Set capabilities for PAM module (optional, for better performance)
    if command -v setcap &> /dev/null; then
        setcap cap_sys_admin+ep "$BINDIR/linuxhello" 2>/dev/null || true
    fi
}

# Print post-installation instructions
print_instructions() {
    echo ""
    echo -e "${GREEN}LinuxHello Installation Complete!${NC}"
    echo "================================"
    echo ""
    echo "Next steps:"
    echo ""
    echo "1. Download ONNX models:"
    echo "   cat $DATADIR/linuxhello/models/README.txt"
    echo ""
    echo "2. Add your user to the video group:"
    echo "   sudo usermod -a -G video \$USER"
    echo "   (Log out and back in for changes to take effect)"
    echo ""
    echo "3. Enroll your face:"
    echo "   sudo linuxhello enroll -user \$USER"
    echo ""
    echo "4. Test authentication:"
    echo "   sudo linuxhello test"
    echo ""
    echo "5. Enable PAM integration (optional):"
    echo "   Edit /etc/pam.d/common-auth or /etc/pam.d/system-auth"
    echo "   Add: auth sufficient pam_linuxhello.so"
    echo "   See: $SYSCONFDIR/linuxhello/common-auth-linuxhello"
    echo ""
    echo "6. Configuration file:"
    echo "   $SYSCONFDIR/linuxhello/linuxhello.conf"
    echo ""
    echo -e "${YELLOW}Warning: Test thoroughly before enabling PAM integration!${NC}"
    echo "A misconfigured PAM can lock you out of your system."
    echo ""
    echo "For help: linuxhello --help"
    echo "For issues: https://github.com/linuxhello/linuxhello/issues"
}

# Uninstall function
uninstall() {
    echo -e "${YELLOW}Uninstalling LinuxHello...${NC}"
    
    rm -f "$BINDIR/linuxhello"
    rm -f "$PAMDIR/pam_linuxhello.so"
    
    echo -e "${YELLOW}Note: Configuration files and data were not removed.${NC}"
    echo "To remove all data, run:"
    echo "  sudo rm -rf $SYSCONFDIR/linuxhello"
    echo "  sudo rm -rf /var/lib/linuxhello"
    echo "  sudo rm -rf $DATADIR/linuxhello"
    
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
            echo "LinuxHello Installation Script"
            echo ""
            echo "Usage: $0 [OPTIONS]"
            echo ""
            echo "Options:"
            echo "  --help, -h       Show this help message"
            echo "  --uninstall, -u  Uninstall LinuxHello"
            echo ""
            echo "Examples:"
            echo "  sudo $0          Install LinuxHello"
            echo "  sudo $0 -u       Uninstall LinuxHello"
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
