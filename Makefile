# LinuxHello - Face Authentication for Linux
# Makefile for building the project

.PHONY: all build clean test install models deps lint fmt vet help setup start-service stop-service

# Variables
BINARY_NAME=linuxhello
ENROLL_BINARY=linuxhello-enroll
GUI_BINARY=linuxhello-gui
TEST_BINARY=linuxhello-test
PAM_MODULE=pam_linuxhello.so
PYTHON_SERVICE=python-service/inference_service.py
PYTHON_VENV=python-service/venv

# Paths
PREFIX?=/usr/local
BINDIR=$(PREFIX)/bin
LIBDIR=$(PREFIX)/lib
SYSCONFDIR=/etc
DATADIR=$(PREFIX)/share
PAMDIR=$(LIBDIR)/security
# System PAM directory (where system PAM modules live)
SYSTEM_PAMDIR?=$(shell if [ -d /lib64/security ]; then echo /lib64/security; elif [ -d /lib/x86_64-linux-gnu/security ]; then echo /lib/x86_64-linux-gnu/security; elif [ -d /lib/security ]; then echo /lib/security; else echo /usr/lib/security; fi)

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
	@echo "📦 Version Management:"
	@echo "  make set-version VERSION=X.Y.Z - Set version across all files"
	@echo "  make get-version - Show current version"
	@echo "  make build-rpm [VERSION=X.Y.Z] - Build RPM package"
	@echo ""
	@echo "🔧 Service Management:"
	@echo "  make setup-after-install - Complete post-installation setup"
	@echo "  make start-services - Start LinuxHello services"
	@echo "  make stop-services  - Stop LinuxHello services"
	@echo "  make status         - Show service status and health"
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

build: build-daemon build-enroll build-test build-gui build-pam ## Build all binaries

build-daemon: ## Build main daemon
	@echo "Building linuxhello daemon..."
	@$(GOBUILD) $(LDFLAGS) -o bin/$(BINARY_NAME) ./cmd/linuxhello

build-enroll: ## Build enrollment tool
	@echo "Building linuxhello-enroll..."
	@$(GOBUILD) $(LDFLAGS) -o bin/$(ENROLL_BINARY) ./cmd/linuxhello-enroll

build-test: ## Build test tool
	@echo "Building linuxhello-test..."
	@$(GOBUILD) $(LDFLAGS) -o bin/$(TEST_BINARY) ./cmd/linuxhello-test

build-gui: ## Build the Management GUI
	@echo "Building web frontend..."
	@cd web-ui && npm install && npm run build
	@echo "Building linuxhello-gui..."
	@$(GOBUILD) $(LDFLAGS) -o bin/$(GUI_BINARY) ./cmd/linuxhello-gui

gui: build-gui ## Build and start the Management GUI (Requires Sudo)
	@echo "Starting Management GUI with sudo privileges..."
	@$(MAKE) start-service-bg
	@echo "Opening LinuxHello Manager at http://localhost:8080"
	@xdg-open http://localhost:8080 2>/dev/null || true
	@sudo ./bin/$(GUI_BINARY) -config configs/linuxhello.conf

sudo-gui: gui ## Alias for gui

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
	@cp -n configs/linuxhello.conf configs/dev/linuxhello.conf 2>/dev/null || true
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
	@./bin/$(ENROLL_BINARY) -config configs/dev/linuxhello.conf -user $(USER) -debug; \
	RESULT=$$?; \
	echo ""; \
	echo "Stopping inference service..."; \
	kill $$(cat logs/inference.pid 2>/dev/null) 2>/dev/null || true; \
	rm -f logs/inference.pid; \
	exit $$RESULT

test-auth: build-test ## Test face authentication
	@echo "Starting inference service..."
	@mkdir -p logs configs/dev data/dev
	@cp -n configs/linuxhello.conf configs/dev/linuxhello.conf 2>/dev/null || true
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
	@./bin/$(TEST_BINARY) -config configs/dev/linuxhello.conf -user $(USER); \
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

install: build-daemon build-enroll build-test build-pam ## Install system-wide
	@echo "Installing LinuxHello..."
	@if [ "$(QUICK_INSTALL)" != "1" ]; then $(MAKE) build-gui; fi
	install -d $(DESTDIR)$(BINDIR)
	install -d $(DESTDIR)$(PAMDIR)
	install -d $(DESTDIR)$(SYSTEM_PAMDIR)
	install -d $(DESTDIR)$(SYSCONFDIR)/linuxhello
	install -d $(DESTDIR)/opt/linuxhello/python-service
	install -d $(DESTDIR)/opt/linuxhello/models
	install -d /var/lib/linuxhello
	install -m 755 bin/$(BINARY_NAME) $(DESTDIR)$(BINDIR)/
	install -m 755 bin/$(ENROLL_BINARY) $(DESTDIR)$(BINDIR)/
	install -m 755 bin/$(TEST_BINARY) $(DESTDIR)$(BINDIR)/
	install -m 755 bin/$(PAM_MODULE) $(DESTDIR)$(PAMDIR)/
	@# Create symlink in system PAM directory so PAM can find the module by name
	@ln -sf $(DESTDIR)$(PAMDIR)/$(PAM_MODULE) $(DESTDIR)$(SYSTEM_PAMDIR)/$(PAM_MODULE) 2>/dev/null || \
		cp bin/$(PAM_MODULE) $(DESTDIR)$(SYSTEM_PAMDIR)/$(PAM_MODULE)
	@echo "PAM module installed to: $(PAMDIR)/$(PAM_MODULE)"
	@echo "PAM module linked/copied to: $(SYSTEM_PAMDIR)/$(PAM_MODULE)"
	install -m 644 configs/linuxhello.conf $(DESTDIR)$(SYSCONFDIR)/linuxhello/
	cp -r python-service/*.py python-service/requirements.txt $(DESTDIR)/opt/linuxhello/python-service/
	cp systemd/linuxhello-inference.service /etc/systemd/system/
	@if [ -f systemd/linuxhello-gui.service ]; then cp systemd/linuxhello-gui.service /etc/systemd/system/; fi
	@if [ -f models/scrfd_person_2.5g.onnx ]; then cp models/*.onnx $(DESTDIR)/opt/linuxhello/models/; fi
	install -m 755 scripts/linuxhello-pam $(DESTDIR)$(BINDIR)/
	@if [ -f bin/$(GUI_BINARY) ]; then install -m 755 bin/$(GUI_BINARY) $(DESTDIR)$(BINDIR)/; fi
	@systemctl daemon-reload 2>/dev/null || true
	@echo ""
	@echo "✅ Installation complete!"
	@echo ""
	@echo "Next steps:"
	@echo "  1. Start the inference service:  sudo systemctl start linuxhello-inference"
	@echo "  2. Enroll your face:             sudo linuxhello-enroll -user \$$USER"
	@echo "  3. Enable PAM for sudo:          sudo linuxhello-pam enable sudo"
	@echo "  4. Test it:                      sudo -k && sudo echo 'Face auth works!'"

uninstall: ## Uninstall LinuxHello
	rm -f $(DESTDIR)$(BINDIR)/$(BINARY_NAME)
	rm -f $(DESTDIR)$(BINDIR)/$(ENROLL_BINARY)
	rm -f $(DESTDIR)$(BINDIR)/$(TEST_BINARY)
	rm -f $(DESTDIR)$(BINDIR)/linuxhello-pam
	rm -f $(DESTDIR)$(PAMDIR)/$(PAM_MODULE)
	rm -f /etc/systemd/system/linuxhello-inference.service
	@echo "Note: Config and data not removed. To fully clean:"
	@echo "  sudo rm -rf $(SYSCONFDIR)/linuxhello /opt/linuxhello /var/lib/linuxhello"

# =============================================================================
# PAM Integration
# =============================================================================

pam-status: ## Show PAM integration status
	@./scripts/linuxhello-pam status

pam-enable-sudo: ## Enable face auth for sudo (safest first step)
	@sudo ./scripts/linuxhello-pam enable sudo

pam-enable-polkit: ## Enable face auth for GUI password dialogs
	@sudo ./scripts/linuxhello-pam enable polkit

pam-enable-sddm: ## Enable face auth for SDDM login (KDE)
	@sudo ./scripts/linuxhello-pam enable sddm

pam-disable-all: ## Disable face auth for all services
	@sudo ./scripts/linuxhello-pam disable sudo
	@sudo ./scripts/linuxhello-pam disable polkit
	@sudo ./scripts/linuxhello-pam disable sddm

pam-restore: ## Restore all PAM configs from backup
	@sudo ./scripts/linuxhello-pam restore

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
# Packaging
# =============================================================================

package: build build-gui ## Build distribution packages (RPM, DEB, tarball)
	@echo "Building distribution packages..."
	@if [ "$(VERSION)" = "" ]; then \
		VERSION=$$(git describe --tags --always 2>/dev/null || echo "dev"); \
	else \
		VERSION=$(VERSION); \
	fi; \
	./packaging/build-packages.sh "$$VERSION"

test-package: package ## Test the built package installation
	@echo "Testing package installation..."
	@if [ -f dist/packages/linuxhello-*.rpm ]; then \
		echo "Testing RPM package:"; \
		rpm -qlp dist/packages/linuxhello-*.rpm; \
	fi
	@if [ -f dist/packages/linuxhello_*.deb ]; then \
		echo "Testing DEB package:"; \
		dpkg -c dist/packages/linuxhello_*.deb; \
	fi

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

# =============================================================================
# Packaging & Distribution
# =============================================================================

package-deps: ## Install packaging dependencies
	@echo "Installing packaging dependencies..."
	@if command -v dnf >/dev/null 2>&1; then \
		sudo dnf install -y rpm-build rpmdevtools; \
	elif command -v apt >/dev/null 2>&1; then \
		sudo apt install -y rpm dpkg-dev; \
	fi

prepare-rpm: build ## Prepare RPM build environment (usage: make prepare-rpm [VERSION=1.2.3])
	@echo "Setting up RPM build environment..."
	@mkdir -p ~/rpmbuild/{BUILD,RPMS,SOURCES,SPECS,SRPMS}
	@if [ "$(VERSION)" != "" ]; then \
		VERSION=$(VERSION); \
	else \
		VERSION=$$(git describe --tags --always 2>/dev/null | sed 's/^v//'); \
		if [ -z "$$VERSION" ]; then VERSION="$$(grep '^Version:' packaging/linuxhello.spec | sed 's/Version:[[:space:]]*//')-$(shell git rev-parse --short HEAD 2>/dev/null || echo 'dev')"; fi; \
	fi; \
	echo "Building version: $$VERSION"; \
	mkdir -p linuxhello-$$VERSION; \
	rsync -av --exclude='.git' --exclude='node_modules' --exclude='dist' \
		--exclude='bin' --exclude='*.tar.gz' --exclude='linuxhello-[0-9]*' ./ linuxhello-$$VERSION/; \
	tar -czf ~/rpmbuild/SOURCES/linuxhello-$$VERSION.tar.gz linuxhello-$$VERSION/; \
	rm -rf linuxhello-$$VERSION; \
	sed "s/Version:.*/Version:        $$VERSION/" packaging/linuxhello.spec > ~/rpmbuild/SPECS/linuxhello.spec

build-rpm: package-deps prepare-rpm ## Build RPM package (usage: make build-rpm [VERSION=1.2.3])
	@echo "Building RPM package..."
	@rpmbuild -ba ~/rpmbuild/SPECS/linuxhello.spec
	@echo "✅ RPM packages built:"
	@find ~/rpmbuild/RPMS ~/rpmbuild/SRPMS -name "*.rpm" -type f

# =============================================================================
# Version Management
# =============================================================================

set-version: ## Set version across all files and create git tag (usage: make set-version VERSION=1.2.3)
	@if [ "$(VERSION)" = "" ]; then \
		echo "❌ VERSION is required. Usage: make set-version VERSION=1.2.3"; \
		exit 1; \
	fi
	@echo "Setting version to $(VERSION)..."
	@sed -i "s/Version:[[:space:]]*.*/Version:        $(VERSION)/" packaging/linuxhello.spec
	@sed -i 's/"version":[[:space:]]*"[^"]*"/"version": "$(VERSION)"/g' web-ui/package.json
	@echo "Creating git tag v$(VERSION)..."
	@git tag -f v$(VERSION) || echo "⚠️  Git tag creation failed (this is okay if git is not available)"
	@echo "✅ Version $(VERSION) set in:"
	@echo "  - packaging/linuxhello.spec"
	@echo "  - web-ui/package.json"
	@echo "  - git tag v$(VERSION)"

get-version: ## Show current version from packaging/linuxhello.spec
	@grep '^Version:' packaging/linuxhello.spec | sed 's/Version:[[:space:]]*//'

test-local: ## Test build locally with act-cli
	@./scripts/test-local.sh

install-rpm: ## Install the built RPM package (after make build-rpm)
	@make build-rpm
	@# Find all RPMs and sort by version number to get the highest version
	@RPM=$$(find ~/rpmbuild/RPMS -name "linuxhello-*.rpm" -type f | while read rpm; do \
		version=$$(basename "$$rpm" | sed 's/linuxhello-\(.*\)-1\.fc.*\.rpm/\1/'); \
		echo "$$version $$rpm"; \
	done | sort -V | tail -1 | cut -d' ' -f2-); \
	if [ -z "$$RPM" ]; then \
		echo "❌ No RPM package found. Run 'make build-rpm' first."; \
		exit 1; \
	fi; \
	echo "Found RPM: $$RPM"; \
	NEW_VERSION=$$(basename "$$RPM" | sed 's/linuxhello-\(.*\)-1\.fc.*\.rpm/\1/'); \
	echo "RPM version: $$NEW_VERSION"; \
	CURRENT_VERSION=$$(rpm -q linuxhello --qf '%{VERSION}' 2>/dev/null || echo "not-installed"); \
	if [ "$$CURRENT_VERSION" != "not-installed" ]; then \
		echo "Currently installed version: $$CURRENT_VERSION"; \
		if [ "$$(printf '%s\n%s' "$$CURRENT_VERSION" "$$NEW_VERSION" | sort -V | tail -1)" = "$$CURRENT_VERSION" ] && [ "$$CURRENT_VERSION" != "$$NEW_VERSION" ]; then \
			echo ""; \
			echo "⚠️  WARNING: Version conflict detected!"; \
			echo "   Currently installed: $$CURRENT_VERSION"; \
			echo "   Trying to install:   $$NEW_VERSION"; \
			echo ""; \
			echo "The version you're trying to install ($$NEW_VERSION) is not higher"; \
			echo "than the currently installed version ($$CURRENT_VERSION)."; \
			echo ""; \
			echo "Solutions:"; \
			echo "  1. Set a higher version: make set-version VERSION=X.Y.Z"; \
			echo "  2. Build with higher version: make build-rpm VERSION=X.Y.Z"; \
			echo "  3. Force install anyway: sudo dnf install -y \"$$RPM\""; \
			echo ""; \
			exit 1; \
		elif [ "$$CURRENT_VERSION" = "$$NEW_VERSION" ]; then \
			echo ""; \
			echo "ℹ️  Same version ($$NEW_VERSION) is already installed."; \
			echo "   This will reinstall the package."; \
			echo ""; \
		else \
			echo "✅ Version upgrade: $$CURRENT_VERSION → $$NEW_VERSION"; \
		fi; \
	else \
		echo "✅ Fresh installation of version $$NEW_VERSION"; \
	fi; \
	echo "Installing $$RPM"; \
	sudo dnf install -y "$$RPM"

# =============================================================================
# Service Management
# =============================================================================

start-services: ## Start LinuxHello services
	@echo "🚀 Starting LinuxHello services..."
	@if sudo systemctl start linuxhello-inference.service; then \
		echo "✅ Inference service started"; \
	else \
		echo "❌ Failed to start inference service"; \
		exit 1; \
	fi
	@if sudo systemctl start linuxhello-gui.service; then \
		echo "✅ GUI service started"; \
	else \
		echo "❌ Failed to start GUI service"; \
		exit 1; \
	fi
	@echo ""
	@echo "🎉 LinuxHello is running!"
	@echo "   • Web interface: http://localhost:8080"
	@echo "   • Check status: make status"

stop-services: ## Stop LinuxHello services
	@echo "🛑 Stopping LinuxHello services..."
	@sudo systemctl stop linuxhello-gui.service || true
	@sudo systemctl stop linuxhello-inference.service || true
	@echo "✅ Services stopped"

restart-services: stop-services start-services ## Restart LinuxHello services

status: ## Show service status and health check
	@echo "📊 LinuxHello Service Status"
	@echo "============================"
	@echo ""
	@echo "🔍 System Services:"
	@systemctl status linuxhello-inference.service --no-pager -l || true
	@echo ""
	@systemctl status linuxhello-gui.service --no-pager -l || true
	@echo ""
	@echo "🌐 Web Interface Test:"
	@if curl -s -o /dev/null -w "%{http_code}" http://localhost:8080 | grep -q "200"; then \
		echo "✅ Web interface accessible at http://localhost:8080"; \
	else \
		echo "❌ Web interface not accessible"; \
	fi

logs: ## Show service logs
	@echo "📝 Recent LinuxHello logs:"
	@echo "=========================="
	@sudo journalctl -u linuxhello-inference -u linuxhello-gui -n 50 --no-pager

setup-after-install: ## Complete post-installation setup
	@echo "🔧 LinuxHello Post-Installation Setup"
	@echo "====================================="
	@echo ""
	@echo "1. Starting services..."
	@make start-services
	@echo ""
	@echo "2. Checking system status..."
	@make status
	@echo ""
	@echo "🎯 Next Steps:"
	@echo "   • Open web interface: http://localhost:8080"
	@echo "   • Enroll your face: linuxhello-enroll"
	@echo "   • Enable PAM auth: linuxhello-pam enable-sudo"
	@echo "   • Test authentication: make test-auth"