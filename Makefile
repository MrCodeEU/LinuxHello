# LinuxHello - Face Authentication for Linux
# Makefile for building the project

.PHONY: all build clean test install models deps lint fmt vet help setup start-service stop-service

# Variables
BINARY_NAME=facelock
ENROLL_BINARY=facelock-enroll
TEST_BINARY=facelock-test
PAM_MODULE=pam_facelock.so
PYTHON_SERVICE=python-service/inference_service.py
PYTHON_VENV=python-service/venv

# Paths
PREFIX?=/usr/local
BINDIR=$(PREFIX)/bin
LIBDIR=$(PREFIX)/lib
SYSCONFDIR=/etc
DATADIR=$(PREFIX)/share
PAMDIR=$(LIBDIR)/security

# Go build flags
GOBUILD=go build
GOCLEAN=go clean
GOTEST=go test

# Build flags
LDFLAGS=-ldflags "-s -w"

# Default target
.DEFAULT_GOAL := help

help: ## Show this help message
	@echo "LinuxHello Build System"
	@echo "======================="
	@echo ""
	@echo "🚀 Quick Start:"
	@echo "  make setup       - Complete setup (Python + dependencies)"
	@echo "  make build       - Build all binaries"
	@echo "  make test-enroll - Enroll your face"
	@echo "  make test-auth   - Test authentication"
	@echo ""
	@echo "Available targets:"
	@grep -E '^[a-zA-Z_-]+:.*?## .*$$' $(MAKEFILE_LIST) | sort | awk 'BEGIN {FS = ":.*?## "}; {printf "  \033[36m%-18s\033[0m %s\n", $$1, $$2}'

all: setup build ## Complete setup and build

# =============================================================================
# Setup
# =============================================================================

setup: setup-python deps models ## Complete setup (Python + Go + Models)
	@echo ""
	@echo "✅ Setup complete!"
	@echo ""
	@echo "Next steps:"
	@echo "  make build       - Build binaries"
	@echo "  make test-enroll - Enroll your face"

setup-python: ## Setup Python inference service
	@echo "Setting up Python inference service..."
	@if [ -f "$$HOME/.ryzen-ai/bin/activate" ]; then \
		echo "✓ Using Ryzen AI environment"; \
		cd python-service && bash -c "source $$HOME/.ryzen-ai/bin/activate && pip install -q -r requirements.txt"; \
	elif [ -f "$(PYTHON_VENV)/bin/python3" ]; then \
		echo "✓ Python venv exists, updating dependencies..."; \
		cd python-service && ./venv/bin/pip install -q -r requirements.txt; \
	else \
		echo "Creating Python venv..."; \
		cd python-service && python3 -m venv venv && ./venv/bin/pip install -q -r requirements.txt; \
	fi
	@echo "✓ Python setup complete"

deps: ## Download Go dependencies
	@echo "Downloading Go dependencies..."
	@go mod download
	@go mod tidy
	@echo "✓ Go dependencies ready"

models: ## Download AI models
	@echo "Downloading AI models..."
	@mkdir -p models
	@if [ ! -f models/scrfd_person_2.5g.onnx ]; then \
		echo "Downloading SCRFD face detection model..."; \
		curl -L -o models/scrfd_person_2.5g.onnx \
			"https://github.com/deepinsight/insightface/releases/download/v0.7/scrfd_person_2.5g.onnx" 2>/dev/null || \
		wget -q -O models/scrfd_person_2.5g.onnx \
			"https://github.com/deepinsight/insightface/releases/download/v0.7/scrfd_person_2.5g.onnx"; \
	else \
		echo "✓ SCRFD model exists"; \
	fi
	@if [ ! -f models/arcface_r50.onnx ]; then \
		echo "Downloading ArcFace recognition model..."; \
		curl -L -o models/arcface_r50.onnx \
			"https://huggingface.co/lithiumice/insightface/resolve/main/models/buffalo_l/w600k_r50.onnx" 2>/dev/null || \
		wget -q -O models/arcface_r50.onnx \
			"https://huggingface.co/lithiumice/insightface/resolve/main/models/buffalo_l/w600k_r50.onnx"; \
	else \
		echo "✓ ArcFace model exists"; \
	fi
	@echo "✓ Models ready"

# =============================================================================
# Build
# =============================================================================

build: build-daemon build-enroll build-test ## Build all binaries

build-daemon: ## Build main daemon
	@echo "Building facelock daemon..."
	@$(GOBUILD) $(LDFLAGS) -o bin/$(BINARY_NAME) ./cmd/facelock

build-enroll: ## Build enrollment tool
	@echo "Building facelock-enroll..."
	@$(GOBUILD) $(LDFLAGS) -o bin/$(ENROLL_BINARY) ./cmd/facelock-enroll

build-test: ## Build test tool
	@echo "Building facelock-test..."
	@$(GOBUILD) $(LDFLAGS) -o bin/$(TEST_BINARY) ./cmd/facelock-test

build-pam: ## Build PAM module
	@echo "Building PAM module..."
	@CGO_ENABLED=1 $(GOBUILD) $(LDFLAGS) -buildmode=c-shared -o bin/$(PAM_MODULE) ./pkg/pam

# =============================================================================
# Service Management
# =============================================================================

start-service: ## Start inference service (foreground)
	@echo "Starting Python inference service..."
	@if [ -f "$$HOME/.ryzen-ai/bin/activate" ]; then \
		cd python-service && bash -c "source $$HOME/.ryzen-ai/bin/activate && python3 inference_service.py"; \
	elif [ -f "$(PYTHON_VENV)/bin/python3" ]; then \
		cd python-service && ./venv/bin/python3 inference_service.py; \
	else \
		echo "❌ No Python environment. Run: make setup"; \
		exit 1; \
	fi

start-service-bg: ## Start inference service (background)
	@mkdir -p logs
	@if [ -f "$$HOME/.ryzen-ai/bin/activate" ]; then \
		cd python-service && bash -c "source $$HOME/.ryzen-ai/bin/activate && nohup python3 inference_service.py > ../logs/inference.log 2>&1 & echo \$$! > ../logs/inference.pid"; \
	elif [ -f "$(PYTHON_VENV)/bin/python3" ]; then \
		cd python-service && nohup ./venv/bin/python3 inference_service.py > ../logs/inference.log 2>&1 & echo $$! > ../logs/inference.pid; \
	else \
		echo "❌ No Python environment. Run: make setup"; \
		exit 1; \
	fi
	@sleep 2
	@echo "✓ Service started (PID: $$(cat logs/inference.pid))"

stop-service: ## Stop inference service
	@if [ -f logs/inference.pid ]; then \
		kill $$(cat logs/inference.pid) 2>/dev/null && echo "✓ Service stopped" || echo "⚠️ Service not running"; \
		rm -f logs/inference.pid; \
	else \
		pkill -f "python.*inference_service.py" 2>/dev/null && echo "✓ Service stopped" || echo "⚠️ No service running"; \
	fi

# =============================================================================
# Testing
# =============================================================================

test-enroll: build-enroll ## Test face enrollment
	@echo "Starting inference service..."
	@mkdir -p logs configs/dev data/dev
	@cp -n configs/facelock.conf configs/dev/facelock.conf 2>/dev/null || true
	@if [ -f "$$HOME/.ryzen-ai/bin/activate" ]; then \
		cd python-service && bash -c "source $$HOME/.ryzen-ai/bin/activate && nohup python3 inference_service.py > ../logs/inference.log 2>&1 & echo \$$! > ../logs/inference.pid"; \
	elif [ -f "$(PYTHON_VENV)/bin/python3" ]; then \
		cd python-service && nohup ./venv/bin/python3 inference_service.py > ../logs/inference.log 2>&1 & echo $$! > ../logs/inference.pid; \
	else \
		echo "❌ No Python environment. Run: make setup"; \
		exit 1; \
	fi
	@echo "Waiting for service to start..."
	@sleep 3
	@echo "Testing enrollment..."
	@./bin/$(ENROLL_BINARY) -config configs/dev/facelock.conf -user $(USER) -debug; \
	RESULT=$$?; \
	echo ""; \
	echo "Stopping inference service..."; \
	kill $$(cat logs/inference.pid 2>/dev/null) 2>/dev/null || true; \
	rm -f logs/inference.pid; \
	exit $$RESULT

test-auth: build-test ## Test face authentication
	@echo "Starting inference service..."
	@mkdir -p logs configs/dev data/dev
	@cp -n configs/facelock.conf configs/dev/facelock.conf 2>/dev/null || true
	@if [ -f "$$HOME/.ryzen-ai/bin/activate" ]; then \
		cd python-service && bash -c "source $$HOME/.ryzen-ai/bin/activate && nohup python3 inference_service.py > ../logs/inference.log 2>&1 & echo \$$! > ../logs/inference.pid"; \
	elif [ -f "$(PYTHON_VENV)/bin/python3" ]; then \
		cd python-service && nohup ./venv/bin/python3 inference_service.py > ../logs/inference.log 2>&1 & echo $$! > ../logs/inference.pid; \
	else \
		echo "❌ No Python environment. Run: make setup"; \
		exit 1; \
	fi
	@echo "Waiting for service to start..."
	@sleep 3
	@echo "Testing authentication..."
	@./bin/$(TEST_BINARY) -config configs/dev/facelock.conf -user $(USER); \
	RESULT=$$?; \
	echo ""; \
	echo "Stopping inference service..."; \
	kill $$(cat logs/inference.pid 2>/dev/null) 2>/dev/null || true; \
	rm -f logs/inference.pid; \
	exit $$RESULT

test: ## Run Go tests
	@echo "Running tests..."
	@$(GOTEST) -v ./...

# =============================================================================
# Code Quality
# =============================================================================

fmt: ## Format Go code
	@go fmt ./...

vet: ## Run go vet
	@go vet ./...

lint: fmt vet ## Run all linters
	@if command -v golangci-lint >/dev/null 2>&1; then \
		golangci-lint run; \
	else \
		echo "golangci-lint not installed, skipping..."; \
	fi

# =============================================================================
# Installation
# =============================================================================

install: build build-pam ## Install system-wide
	@echo "Installing LinuxHello..."
	install -d $(DESTDIR)$(BINDIR)
	install -d $(DESTDIR)$(PAMDIR)
	install -d $(DESTDIR)$(SYSCONFDIR)/facelock
	install -d $(DESTDIR)/opt/facelock/python-service
	install -d $(DESTDIR)/opt/facelock/models
	install -m 755 bin/$(BINARY_NAME) $(DESTDIR)$(BINDIR)/
	install -m 755 bin/$(ENROLL_BINARY) $(DESTDIR)$(BINDIR)/
	install -m 755 bin/$(TEST_BINARY) $(DESTDIR)$(BINDIR)/
	install -m 755 bin/$(PAM_MODULE) $(DESTDIR)$(PAMDIR)/
	install -m 644 configs/facelock.conf $(DESTDIR)$(SYSCONFDIR)/facelock/
	cp -r python-service/*.py python-service/requirements.txt $(DESTDIR)/opt/facelock/python-service/
	cp systemd/facelock-inference.service /etc/systemd/system/
	@if [ -f models/scrfd_person_2.5g.onnx ]; then cp models/*.onnx $(DESTDIR)/opt/facelock/models/; fi
	@echo ""
	@echo "✅ Installation complete!"
	@echo ""
	@echo "Next steps:"
	@echo "  sudo systemctl start facelock-inference"
	@echo "  sudo systemctl enable facelock-inference"
	@echo "  facelock-enroll -user $$USER"

uninstall: ## Uninstall LinuxHello
	rm -f $(DESTDIR)$(BINDIR)/$(BINARY_NAME)
	rm -f $(DESTDIR)$(BINDIR)/$(ENROLL_BINARY)
	rm -f $(DESTDIR)$(BINDIR)/$(TEST_BINARY)
	rm -f $(DESTDIR)$(PAMDIR)/$(PAM_MODULE)
	rm -f /etc/systemd/system/facelock-inference.service
	@echo "Note: Config and data not removed. To fully clean:"
	@echo "  sudo rm -rf $(SYSCONFDIR)/facelock /opt/facelock /var/lib/facelock"

# =============================================================================
# Cleanup
# =============================================================================

clean: ## Clean build artifacts
	@echo "Cleaning..."
	@$(GOCLEAN)
	@rm -rf bin/
	@rm -f coverage.out coverage.html

clean-all: clean ## Clean everything including venv
	@rm -rf $(PYTHON_VENV)
	@rm -rf python-service/__pycache__
	@rm -rf logs/ debug_enrollment/
	@rm -rf data/dev/ configs/dev/

# =============================================================================
# Development
# =============================================================================

dev-deps: ## Install development dependencies (Fedora)
	@echo "Installing development dependencies..."
	@if command -v dnf >/dev/null 2>&1; then \
		sudo dnf install -y golang gcc libv4l-devel pam-devel sqlite-devel python3-devel; \
	elif command -v apt >/dev/null 2>&1; then \
		sudo apt update && sudo apt install -y golang gcc libv4l-dev libpam0g-dev libsqlite3-dev python3-dev python3-venv; \
	elif command -v pacman >/dev/null 2>&1; then \
		sudo pacman -S --needed go gcc v4l-utils pam sqlite python; \
	fi

# =============================================================================
# Release
# =============================================================================

release: clean build ## Build release binaries
	@mkdir -p dist
	@cp bin/* dist/
	@echo "Release binaries in dist/"
