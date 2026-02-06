# LinuxHello Distribution Guide

## Overview

LinuxHello can be distributed as RPM packages for Red Hat-based systems (Fedora, RHEL, Rocky Linux, etc.).

## Building Packages

### Local Build

```bash
# Install build dependencies
make dev-deps

# Build RPM package
make build-rpm

# Install the package
make install-rpm
```

### Test with act-cli

```bash
# Test GitHub workflow locally
make test-local

# Or run act directly
act workflow_dispatch -v
```

### GitHub Actions

The project includes automated builds that create RPM packages on every tag:

1. **Create a release tag:**
   ```bash
   git tag v1.0.0
   git push origin v1.0.0
   ```

2. **GitHub Actions will automatically:**
   - Build RPM packages
   - Create a GitHub release
   - Upload packages as release assets

## Installation Flow

After installing the RPM package, users can:

1. **Start inference service:**
   ```bash
   sudo systemctl start linuxhello-inference
   sudo systemctl enable linuxhello-inference
   ```

2. **Launch the desktop GUI:**
   ```bash
   sudo linuxhello
   ```

3. **Complete setup through GUI:**
   - **Enroll Face tab:** Enroll user faces
   - **System & PAM tab:** Enable PAM authentication for sudo/login
   - **Auth Test tab:** Test face detection and authentication
   - **Configuration tab:** Adjust settings

## Package Contents

The RPM includes:
- Single `linuxhello` binary (Wails app with GUI, daemon, enroll, and test subcommands)
- PAM module (`pam_linuxhello.so`)
- Python inference service
- Systemd service files
- Configuration files

## Dependencies

Runtime dependencies (automatically installed):
- `pam` - PAM authentication
- `sqlite` - User database
- `python3`, `python3-pip` - Python inference service
- `systemd` - Service management
- `polkit` - Privilege escalation

## Post-Installation

The package automatically:
- Creates `linuxhello` user account
- Sets up data directories with proper permissions
- Installs and enables systemd services
- Provides setup instructions

Users just need to:
1. Open the web GUI
2. Enroll their faces
3. Enable PAM authentication
4. Start using face authentication!