# LinuxHello - Face Authentication for Linux

.PHONY: all build clean test install help setup gui models

# Variables
BINARY_NAME=linuxhello
PAM_MODULE=pam_linuxhello.so
PYTHON_VENV=python-service/venv

# Paths
PREFIX?=/usr/local
BINDIR=$(PREFIX)/bin
LIBDIR=$(PREFIX)/lib
SYSCONFDIR=/etc
PAMDIR=$(LIBDIR)/security
SYSTEM_PAMDIR?=$(shell if [ -d /lib64/security ]; then echo /lib64/security; elif [ -d /lib/x86_64-linux-gnu/security ]; then echo /lib/x86_64-linux-gnu/security; elif [ -d /lib/security ]; then echo /lib/security; else echo /usr/lib/security; fi)

# Go
GOBUILD=go build
LDFLAGS=-ldflags "-s -w"

# Default target
.DEFAULT_GOAL := help

help: ## Show this help message
	@echo "LinuxHello Build System"
	@echo "======================="
	@echo ""
	@echo "Quick Start:"
	@echo "  make setup       - Complete setup (Python + Go deps + models)"
	@echo "  make build       - Build linuxhello + PAM module"
	@echo "  make gui         - Build and launch the desktop GUI"
	@echo "  make test-enroll - Enroll your face"
	@echo "  make test-auth   - Test authentication"
	@echo ""
	@echo "All targets:"
	@grep -E '^[a-zA-Z_-]+:.*?## .*$$' $(MAKEFILE_LIST) | sort | awk 'BEGIN {FS = ":.*?## "}; {printf "  \033[36m%-18s\033[0m %s\n", $$1, $$2}'

all: setup build ## Complete setup and build

# =============================================================================
# Setup
# =============================================================================

setup: setup-python deps models ## Complete setup (Python + Go + Models)
	@echo ""
	@echo "Setup complete! Next: make build"

setup-python: ## Setup Python inference service
	@echo "Setting up Python inference service..."
	@if [ -f "$$HOME/.ryzen-ai/bin/activate" ]; then \
		echo "Using Ryzen AI environment"; \
		cd python-service && bash -c "source $$HOME/.ryzen-ai/bin/activate && pip install -q -r requirements.txt"; \
	elif [ -f "$(PYTHON_VENV)/bin/python3" ]; then \
		echo "Python venv exists, updating dependencies..."; \
		cd python-service && ./venv/bin/pip install -q -r requirements.txt; \
	else \
		echo "Creating Python venv..."; \
		cd python-service && python3 -m venv venv && ./venv/bin/pip install -q -r requirements.txt; \
	fi

deps: ## Download Go dependencies
	@go mod download
	@go mod tidy

models: ## Download AI models
	@mkdir -p models
	@if [ ! -f models/det_10g.onnx ]; then \
		echo "Downloading SCRFD face detection model (det_10g from buffalo_l)..."; \
		if curl -L -o /tmp/buffalo_l.zip \
			"https://huggingface.co/public-data/insightface/resolve/main/models/buffalo_l.zip" 2>/dev/null; then \
			unzip -o -j /tmp/buffalo_l.zip det_10g.onnx -d models/ && rm -f /tmp/buffalo_l.zip; \
		elif wget -q -O /tmp/buffalo_l.zip \
			"https://huggingface.co/public-data/insightface/resolve/main/models/buffalo_l.zip"; then \
			unzip -o -j /tmp/buffalo_l.zip det_10g.onnx -d models/ && rm -f /tmp/buffalo_l.zip; \
		else \
			echo "Failed to download det_10g.onnx. Install curl or wget."; \
			exit 1; \
		fi; \
	fi
	@if [ ! -f models/arcface_r50.onnx ]; then \
		echo "Downloading ArcFace recognition model..."; \
		curl -L -o models/arcface_r50.onnx \
			"https://huggingface.co/public-data/insightface/resolve/main/models/buffalo_l/w600k_r50.onnx" 2>/dev/null || \
		wget -q -O models/arcface_r50.onnx \
			"https://huggingface.co/public-data/insightface/resolve/main/models/buffalo_l/w600k_r50.onnx"; \
	fi

# =============================================================================
# Build
# =============================================================================

build: build-app build-pam ## Build all binaries

build-app: ## Build linuxhello (Wails app with all subcommands)
	@echo "Building frontend assets..."
	@cd frontend && npm install && npm run build
	@echo "Building linuxhello..."
	@$(GOBUILD) $(LDFLAGS) -tags desktop,production -o bin/$(BINARY_NAME) .

build-pam: ## Build PAM module
	@echo "Building PAM module..."
	@CGO_ENABLED=1 $(GOBUILD) $(LDFLAGS) -buildmode=c-shared -o bin/$(PAM_MODULE) ./pkg/pam

# =============================================================================
# Development
# =============================================================================

gui: build-app ## Build and launch the desktop GUI (requires sudo)
	@$(MAKE) stop-service
	@$(MAKE) start-service-bg
	@sudo ./bin/$(BINARY_NAME)

start-service-bg: ## Start inference service (background)
	@mkdir -p logs
	@if [ -f "$$HOME/.ryzen-ai/bin/activate" ]; then \
		. $$HOME/.ryzen-ai/bin/activate && \
		cd python-service && \
		nohup python3 inference_service.py > $(CURDIR)/logs/inference.log 2>&1 & echo $$! > $(CURDIR)/logs/inference.pid; \
	elif [ -f "$(PYTHON_VENV)/bin/python3" ]; then \
		cd python-service && \
		nohup ./venv/bin/python3 inference_service.py > $(CURDIR)/logs/inference.log 2>&1 & echo $$! > $(CURDIR)/logs/inference.pid; \
	else \
		echo "No Python environment. Run: make setup"; \
		exit 1; \
	fi
	@sleep 2
	@echo "Service started (PID: $$(cat logs/inference.pid))"

stop-service: ## Stop inference service
	@pkill -f "[p]ython3.*inference_service" 2>/dev/null; \
	rm -f logs/inference.pid; \
	echo "Service stopped"

dev-deps: ## Install development dependencies
	@if command -v dnf >/dev/null 2>&1; then \
		sudo dnf install -y golang gcc libv4l-devel pam-devel sqlite-devel python3-devel; \
	elif command -v apt >/dev/null 2>&1; then \
		sudo apt update && sudo apt install -y golang gcc libv4l-dev libpam0g-dev libsqlite3-dev python3-dev python3-venv; \
	elif command -v pacman >/dev/null 2>&1; then \
		sudo pacman -S --needed go gcc v4l-utils pam sqlite python; \
	fi

# =============================================================================
# Testing
# =============================================================================

test: ## Run Go unit tests
	@go test -v ./...

test-enroll: build-app ## Test face enrollment
	@mkdir -p logs configs/dev data/dev
	@cp -n configs/linuxhello.conf configs/dev/linuxhello.conf 2>/dev/null || true
	@if [ -f "$$HOME/.ryzen-ai/bin/activate" ]; then \
		cd python-service && bash -c "source $$HOME/.ryzen-ai/bin/activate && nohup python3 inference_service.py > ../logs/inference.log 2>&1 & echo \$$! > ../logs/inference.pid"; \
	elif [ -f "$(PYTHON_VENV)/bin/python3" ]; then \
		cd python-service && nohup ./venv/bin/python3 inference_service.py > ../logs/inference.log 2>&1 & echo $$! > ../logs/inference.pid; \
	else \
		echo "No Python environment. Run: make setup"; \
		exit 1; \
	fi
	@sleep 3
	@./bin/$(BINARY_NAME) enroll -config configs/dev/linuxhello.conf -user $(USER) -debug; \
	RESULT=$$?; \
	kill $$(cat logs/inference.pid 2>/dev/null) 2>/dev/null || true; \
	rm -f logs/inference.pid; \
	exit $$RESULT

test-auth: build-app ## Test face authentication
	@mkdir -p logs configs/dev data/dev
	@cp -n configs/linuxhello.conf configs/dev/linuxhello.conf 2>/dev/null || true
	@if [ -f "$$HOME/.ryzen-ai/bin/activate" ]; then \
		cd python-service && bash -c "source $$HOME/.ryzen-ai/bin/activate && nohup python3 inference_service.py > ../logs/inference.log 2>&1 & echo \$$! > ../logs/inference.pid"; \
	elif [ -f "$(PYTHON_VENV)/bin/python3" ]; then \
		cd python-service && nohup ./venv/bin/python3 inference_service.py > ../logs/inference.log 2>&1 & echo $$! > ../logs/inference.pid; \
	else \
		echo "No Python environment. Run: make setup"; \
		exit 1; \
	fi
	@sleep 3
	@./bin/$(BINARY_NAME) test -config configs/dev/linuxhello.conf -user $(USER); \
	RESULT=$$?; \
	kill $$(cat logs/inference.pid 2>/dev/null) 2>/dev/null || true; \
	rm -f logs/inference.pid; \
	exit $$RESULT

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

install: build ## Install system-wide
	install -d $(DESTDIR)$(BINDIR)
	install -d $(DESTDIR)$(PAMDIR)
	install -d $(DESTDIR)$(SYSTEM_PAMDIR)
	install -d $(DESTDIR)$(SYSCONFDIR)/linuxhello
	install -d $(DESTDIR)/opt/linuxhello/python-service
	install -d $(DESTDIR)/opt/linuxhello/models
	install -d /var/lib/linuxhello
	install -m 755 bin/$(BINARY_NAME) $(DESTDIR)$(BINDIR)/
	install -m 755 bin/$(PAM_MODULE) $(DESTDIR)$(PAMDIR)/
	@ln -sf $(DESTDIR)$(PAMDIR)/$(PAM_MODULE) $(DESTDIR)$(SYSTEM_PAMDIR)/$(PAM_MODULE) 2>/dev/null || \
		cp bin/$(PAM_MODULE) $(DESTDIR)$(SYSTEM_PAMDIR)/$(PAM_MODULE)
	install -m 644 configs/linuxhello.conf $(DESTDIR)$(SYSCONFDIR)/linuxhello/
	cp -r python-service/*.py python-service/requirements.txt $(DESTDIR)/opt/linuxhello/python-service/
	cp systemd/linuxhello-inference.service /etc/systemd/system/
	@if [ -f models/det_10g.onnx ]; then cp models/*.onnx $(DESTDIR)/opt/linuxhello/models/; fi
	install -m 755 scripts/linuxhello-pam $(DESTDIR)$(BINDIR)/
	@systemctl daemon-reload 2>/dev/null || true
	@echo ""
	@echo "Installed! Next steps:"
	@echo "  sudo systemctl start linuxhello-inference"
	@echo "  sudo linuxhello enroll -user \$$USER"
	@echo "  sudo linuxhello-pam enable sudo"
	@echo "  sudo linuxhello   # Launch GUI"

uninstall: ## Uninstall LinuxHello
	rm -f $(DESTDIR)$(BINDIR)/$(BINARY_NAME)
	rm -f $(DESTDIR)$(BINDIR)/linuxhello-pam
	rm -f $(DESTDIR)$(PAMDIR)/$(PAM_MODULE)
	rm -f $(DESTDIR)$(SYSTEM_PAMDIR)/$(PAM_MODULE)
	rm -f /etc/systemd/system/linuxhello-inference.service
	@systemctl daemon-reload 2>/dev/null || true
	@echo "Note: Config and data not removed. To fully clean:"
	@echo "  sudo rm -rf $(SYSCONFDIR)/linuxhello /opt/linuxhello /var/lib/linuxhello"

# =============================================================================
# PAM Integration
# =============================================================================

pam-status: ## Show PAM integration status
	@./scripts/linuxhello-pam status

pam-enable-sudo: ## Enable face auth for sudo
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
# Packaging
# =============================================================================

set-version: ## Set version across all files (usage: make set-version VERSION=1.2.3)
	@if [ "$(VERSION)" = "" ]; then \
		echo "VERSION is required. Usage: make set-version VERSION=1.2.3"; \
		exit 1; \
	fi
	@echo "Setting version to $(VERSION)..."
	@sed -i "s/Version:[[:space:]]*.*/Version:        $(VERSION)/" packaging/linuxhello.spec
	@sed -i "s/Version:[[:space:]]*.*/Version:        $(VERSION)/" linuxhello.spec
	@sed -i 's/"version":[[:space:]]*"[^"]*"/"version": "$(VERSION)"/g' frontend/package.json
	@sed -i 's/"version":[[:space:]]*"[^"]*"/"version": "$(VERSION)"/g' wails.json
	@git tag -f v$(VERSION) || echo "Git tag creation failed"
	@echo "Version $(VERSION) set in spec files, package.json, wails.json, and git tag"

get-version: ## Show current version
	@grep '^Version:' packaging/linuxhello.spec | sed 's/Version:[[:space:]]*//'

package: build ## Build distribution packages (RPM, DEB, tarball)
	@VERSION=$${VERSION:-$$(git describe --tags --always 2>/dev/null || echo "dev")}; \
	./packaging/build-packages.sh "$$VERSION"

build-rpm: build ## Build RPM package (usage: make build-rpm [VERSION=1.2.3])
	@if command -v rpmbuild >/dev/null 2>&1; then true; \
	elif command -v dnf >/dev/null 2>&1; then sudo dnf install -y rpm-build rpmdevtools; \
	elif command -v apt >/dev/null 2>&1; then sudo apt install -y rpm dpkg-dev; \
	fi
	@mkdir -p ~/rpmbuild/{BUILD,RPMS,SOURCES,SPECS,SRPMS}
	@if [ "$(VERSION)" != "" ]; then \
		VERSION=$(VERSION); \
	else \
		VERSION=$$(git describe --tags --always 2>/dev/null | sed 's/^v//'); \
		if [ -z "$$VERSION" ]; then VERSION="$$(grep '^Version:' packaging/linuxhello.spec | sed 's/Version:[[:space:]]*//')"; fi; \
	fi; \
	echo "Building RPM version: $$VERSION"; \
	mkdir -p linuxhello-$$VERSION; \
	rsync -av --exclude='.git' --exclude='node_modules' --exclude='dist' \
		--exclude='bin' --exclude='*.tar.gz' --exclude='linuxhello-[0-9]*' ./ linuxhello-$$VERSION/; \
	tar -czf ~/rpmbuild/SOURCES/linuxhello-$$VERSION.tar.gz linuxhello-$$VERSION/; \
	rm -rf linuxhello-$$VERSION; \
	sed "s/Version:.*/Version:        $$VERSION/" packaging/linuxhello.spec > ~/rpmbuild/SPECS/linuxhello.spec
	@rpmbuild -ba ~/rpmbuild/SPECS/linuxhello.spec
	@echo "RPM packages built:"
	@find ~/rpmbuild/RPMS ~/rpmbuild/SRPMS -name "*.rpm" -type f

install-rpm: build-rpm ## Build and install the RPM package
	@RPM=$$(find ~/rpmbuild/RPMS -name "linuxhello-*.rpm" -type f | while read rpm; do \
		version=$$(basename "$$rpm" | sed 's/linuxhello-\(.*\)-1\.fc.*\.rpm/\1/'); \
		echo "$$version $$rpm"; \
	done | sort -V | tail -1 | cut -d' ' -f2-); \
	if [ -z "$$RPM" ]; then \
		echo "No RPM package found."; \
		exit 1; \
	fi; \
	echo "Installing $$RPM"; \
	sudo dnf install -y "$$RPM"

# =============================================================================
# System Services (post-install)
# =============================================================================

start-services: ## Start LinuxHello systemd services
	@sudo systemctl start linuxhello-inference.service
	@echo "Inference service started. Launch GUI: sudo linuxhello"

stop-services: ## Stop LinuxHello systemd services
	@sudo systemctl stop linuxhello-inference.service || true
	@echo "Services stopped"

status: ## Show service status
	@systemctl status linuxhello-inference.service --no-pager -l || true

logs: ## Show service logs
	@sudo journalctl -u linuxhello-inference -n 50 --no-pager

# =============================================================================
# Cleanup
# =============================================================================

clean: ## Clean build artifacts
	@go clean
	@rm -rf bin/

clean-all: clean ## Clean everything including venv and dev data
	@rm -rf $(PYTHON_VENV) python-service/__pycache__
	@rm -rf logs/ debug_enrollment/ data/dev/ configs/dev/
