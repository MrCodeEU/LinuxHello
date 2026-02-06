#!/bin/bash
# Setup script for LinuxHello hybrid architecture

set -e

echo "LinuxHello Hybrid Setup"
echo "===================="
echo ""

# Check Python version
if ! command -v python3 &> /dev/null; then
    echo "Error: Python 3 is required but not installed"
    exit 1
fi

PYTHON_VERSION=$(python3 --version | cut -d' ' -f2 | cut -d'.' -f1-2)
PYTHON_MAJOR=$(echo $PYTHON_VERSION | cut -d'.' -f1)
PYTHON_MINOR=$(echo $PYTHON_VERSION | cut -d'.' -f2)

echo "✓ Python $PYTHON_VERSION detected"

# Check if Python version is too new (ONNX Runtime supports up to 3.12)
if [ "$PYTHON_MAJOR" -eq 3 ] && [ "$PYTHON_MINOR" -gt 12 ]; then
    echo "⚠️  WARNING: Python $PYTHON_VERSION is too new!"
    echo "   ONNX Runtime currently supports Python 3.8-3.12"
    echo ""
    echo "Options:"
    echo "  1. Install Python 3.12: sudo apt install python3.12 python3.12-venv"
    echo "  2. Use system Python 3.12: python3.12 -m venv python-service/venv"
    echo "  3. Continue anyway (may fail)"
    echo ""
    read -p "Continue anyway? (y/N) " -n 1 -r
    echo
    if [[ ! $REPLY =~ ^[Yy]$ ]]; then
        exit 1
    fi
fi

# Detect hardware acceleration
ONNX_RUNTIME_PKG="onnxruntime"

# Check if Ryzen AI environment is active
USING_RYZEN_AI=false
if [ ! -z "$RYZEN_AI_INSTALLATION_PATH" ]; then
    echo "✓ AMD Ryzen AI environment active"
    echo "  Path: $RYZEN_AI_INSTALLATION_PATH"
    ONNX_RUNTIME_PKG="skip"  # Use Ryzen AI's own ONNX Runtime
    USING_RYZEN_AI=true
elif [ -f "$HOME/.ryzen-ai/bin/activate" ]; then
    echo "✓ AMD Ryzen AI detected at $HOME/.ryzen-ai"
    echo "  Please activate it first:"
    echo "  source $HOME/.ryzen-ai/bin/activate"
    echo "  Then run this setup again"
    exit 1
# Check for DirectML (Ryzen AI NPU / Windows)
elif [ -d "/sys/class/dma" ] && grep -qi "amd" /proc/cpuinfo; then
    echo "✓ AMD Ryzen CPU detected - checking for AI NPU support"
    # Check if we're on Windows (WSL) or Linux with DirectML support
    if grep -qi microsoft /proc/version; then
        echo "✓ WSL detected - using DirectML for Ryzen AI NPU"
        ONNX_RUNTIME_PKG="onnxruntime-directml"
    elif [ -f "/usr/lib/libdirectml.so" ] || [ -f "/usr/local/lib/libdirectml.so" ]; then
        echo "✓ DirectML library found - using DirectML for Ryzen AI NPU"
        ONNX_RUNTIME_PKG="onnxruntime-directml"
    else
        echo "ℹ️  Ryzen AI NPU detected but DirectML not available"
        echo "  For NPU support on Linux, install: https://www.amd.com/en/developer/ryzen-ai.html"
        ONNX_RUNTIME_PKG="onnxruntime"
    fi
# Check for ROCm (AMD GPU)
elif command -v rocminfo &> /dev/null; then
    echo "✓ ROCm detected - using ROCm for AMD GPU acceleration"
    ONNX_RUNTIME_PKG="onnxruntime-rocm"
elif lspci 2>/dev/null | grep -i "amd.*vga" > /dev/null; then
    echo "⚠️  AMD GPU detected but ROCm not installed"
    echo "  Install ROCm for GPU acceleration: https://rocm.docs.amd.com/"
    ONNX_RUNTIME_PKG="onnxruntime"
# Check for NVIDIA GPU
elif command -v nvidia-smi &> /dev/null; then
    echo "✓ NVIDIA GPU detected - using CUDA"
    ONNX_RUNTIME_PKG="onnxruntime-gpu"
else
    echo "ℹ️  CPU-only mode (no GPU/NPU acceleration detected)"
    ONNX_RUNTIME_PKG="onnxruntime"
fi

# Setup Python environment
cd python-service
echo ""

if [ "$USING_RYZEN_AI" = true ]; then
    echo "Using Ryzen AI Python environment (no venv needed)..."
    # Verify Ryzen AI environment has necessary Python
    if ! command -v python3 &> /dev/null; then
        echo "❌ Error: python3 not found in Ryzen AI environment"
        exit 1
    fi
else
    echo "Creating Python virtual environment..."
    python3 -m venv venv
    # Activate virtual environment
    source venv/bin/activate
fi

echo "Installing Python dependencies..."
pip install --upgrade pip

# Install appropriate ONNX Runtime
echo ""
if [ "$ONNX_RUNTIME_PKG" = "skip" ]; then
    echo "Using Ryzen AI ONNX Runtime (already installed)"
    echo "Skipping ONNX Runtime installation..."
else
    echo "Installing ONNX Runtime ($ONNX_RUNTIME_PKG)..."
    if [ "$ONNX_RUNTIME_PKG" = "onnxruntime-rocm" ]; then
    pip install onnxruntime-rocm>=1.16.0 || {
        echo "⚠️  ROCm installation failed, falling back to CPU"
        pip install onnxruntime>=1.16.0
    }
elif [ "$ONNX_RUNTIME_PKG" = "onnxruntime-directml" ]; then
    pip install onnxruntime-directml>=1.16.0 || {
        echo "⚠️  DirectML installation failed, falling back to CPU"
        pip install onnxruntime>=1.16.0
    }
elif [ "$ONNX_RUNTIME_PKG" = "onnxruntime-gpu" ]; then
    pip install onnxruntime-gpu>=1.16.0 || {
        echo "⚠️  GPU installation failed, falling back to CPU"
        pip install onnxruntime>=1.16.0
    }
else
    pip install onnxruntime>=1.16.0 || {
        echo "❌ Failed to install ONNX Runtime"
        echo ""
        echo "This might be because Python $PYTHON_VERSION is too new."
        echo "Try with Python 3.12:"
        echo "  python3.12 -m venv python-service/venv"
        echo "  source python-service/venv/bin/activate"
fi
        echo "  pip install onnxruntime>=1.16.0"
        exit 1
    }
fi

# Install other dependencies
echo ""
echo "Installing other dependencies..."
pip install -r requirements.txt || {
    echo "❌ Failed to install Python dependencies"
    echo "Check requirements.txt and try manually"
    exit 1
}

echo ""
echo "Generating gRPC code..."
python3 -m grpc_tools.protoc \
    -I../api \
    --python_out=. \
    --grpc_python_out=. \
    ../api/inference.proto

echo ""
echo "✓ Python service setup complete"

# Go back to root
cd ..

# Generate Go gRPC code
echo ""
echo "Setting up Go gRPC client..."

# Install protoc-gen-go if needed
if ! command -v protoc-gen-go &> /dev/null; then
    echo "Installing protoc-gen-go..."
    go install google.golang.org/protobuf/cmd/protoc-gen-go@latest
fi

if ! command -v protoc-gen-go-grpc &> /dev/null; then
    echo "Installing protoc-gen-go-grpc..."
    go install google.golang.org/grpc/cmd/protoc-gen-go-grpc@latest
fi

# Generate Go code
mkdir -p api/inference
protoc \
    --go_out=. --go_opt=paths=source_relative \
    --go-grpc_out=. --go-grpc_opt=paths=source_relative \
    api/inference.proto

# Update Go dependencies
echo "Updating Go dependencies..."
go mod tidy

echo ""
echo "✓ Go client setup complete"

echo ""
echo "===================="
echo "✅ Setup Complete!"
echo ""
echo "Hardware Configuration:"
echo "  Python:  $PYTHON_VERSION"
echo "  Runtime: $ONNX_RUNTIME_PKG"
echo ""
# Verify installation
if cd python-service && source venv/bin/activate && python3 -c "import onnxruntime as ort; print('  Providers:', ort.get_available_providers())" 2>/dev/null; then
    cd ..
else
    cd ..
    echo "  ⚠️  Could not verify ONNX Runtime installation"
fi
echo ""
echo "Next steps:"
echo "1. Start the Python inference service:"
echo "   cd python-service && source venv/bin/activate"
echo "   python3 inference_service.py"
echo ""
echo "2. In another terminal, run LinuxHello:"
echo "   make build && sudo ./bin/linuxhello enroll -user yourname"
echo ""
echo "Or use: make start-service (Terminal 1) && make test-enroll (Terminal 2)"
echo "===================="
