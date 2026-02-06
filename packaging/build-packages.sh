#!/bin/bash
# Build script for creating distribution packages

set -e

VERSION=${1:-$(git describe --tags --always)}
ARCH=${2:-$(uname -m)}
BUILD_DIR="/tmp/linuxhello-build"
PACKAGE_DIR="$(pwd)/dist/packages"

echo "Building LinuxHello packages for version $VERSION on $ARCH"

# Clean up
rm -rf "$BUILD_DIR" "$PACKAGE_DIR"
mkdir -p "$BUILD_DIR" "$PACKAGE_DIR"

# Create source tarball
echo "Creating source tarball..."
git archive --format=tar.gz --prefix="linuxhello-$VERSION/" HEAD > "$BUILD_DIR/linuxhello-$VERSION.tar.gz"

# Build RPM
if command -v rpmbuild >/dev/null 2>&1; then
    echo "Building RPM package..."
    
    # Set up RPM build environment
    RPMBUILD_DIR="$BUILD_DIR/rpmbuild"
    mkdir -p "$RPMBUILD_DIR"/{BUILD,RPMS,SOURCES,SPECS,SRPMS}
    
    # Copy source and spec
    cp "$BUILD_DIR/linuxhello-$VERSION.tar.gz" "$RPMBUILD_DIR/SOURCES/"
    cp linuxhello.spec "$RPMBUILD_DIR/SPECS/"
    
    # Update version in spec file
    sed -i "s/Version:.*/Version:        $VERSION/" "$RPMBUILD_DIR/SPECS/linuxhello.spec"
    
    # Build RPM
    rpmbuild --define "_topdir $RPMBUILD_DIR" \
             --define "_version $VERSION" \
             -ba "$RPMBUILD_DIR/SPECS/linuxhello.spec"
    
    # Copy RPMs to package directory
    find "$RPMBUILD_DIR/RPMS" -name "*.rpm" -exec cp {} "$PACKAGE_DIR/" \;
    find "$RPMBUILD_DIR/SRPMS" -name "*.rpm" -exec cp {} "$PACKAGE_DIR/" \;
    
    echo "RPM packages created in $PACKAGE_DIR"
else
    echo "rpmbuild not found, skipping RPM build"
fi

# Build DEB (basic, for broader compatibility)
if command -v dpkg-deb >/dev/null 2>&1; then
    echo "Building DEB package..."
    
    DEB_DIR="$BUILD_DIR/linuxhello-deb"
    mkdir -p "$DEB_DIR/DEBIAN"
    mkdir -p "$DEB_DIR/usr/bin"
    mkdir -p "$DEB_DIR/usr/lib/security"
    mkdir -p "$DEB_DIR/etc/systemd/system"
    mkdir -p "$DEB_DIR/etc/linuxhello"
    mkdir -p "$DEB_DIR/usr/share/linuxhello"
    mkdir -p "$DEB_DIR/var/lib/linuxhello"
    
    # Create control file
    cat > "$DEB_DIR/DEBIAN/control" << EOF
Package: linuxhello
Version: $VERSION
Section: admin
Priority: optional
Architecture: amd64
Depends: pam-modules, libsqlite3-0, libv4l-0, python3, python3-pip, python3-venv
Maintainer: MrCode <mrcode@example.com>
Description: Face authentication system for Linux
 LinuxHello is a modern face authentication system that integrates with PAM
 to provide secure, contactless authentication using facial recognition.
EOF
    
    # Install binaries (assumes they're already built)
    if [ -f "bin/linuxhello" ]; then
        cp bin/linuxhello "$DEB_DIR/usr/bin/"
        cp bin/pam_linuxhello.so "$DEB_DIR/usr/lib/security/" 2>/dev/null || true
        cp configs/linuxhello.conf "$DEB_DIR/etc/linuxhello/"
        cp systemd/linuxhello-inference.service "$DEB_DIR/etc/systemd/system/"
        cp scripts/linuxhello-pam "$DEB_DIR/usr/bin/"
        cp -r python-service "$DEB_DIR/usr/share/linuxhello/"
        # Note: frontend is embedded in linuxhello binary, no separate install needed
        
        # Build DEB
        dpkg-deb --build "$DEB_DIR" "$PACKAGE_DIR/linuxhello_${VERSION}_amd64.deb"
        echo "DEB package created: linuxhello_${VERSION}_amd64.deb"
    else
        echo "Binaries not found, skipping DEB build"
    fi
fi

# Create generic tarball
echo "Creating generic tarball..."
TARBALL_DIR="$BUILD_DIR/linuxhello-generic"
mkdir -p "$TARBALL_DIR"

# Copy built files if they exist
if [ -f "bin/linuxhello" ]; then
    cp -r bin "$TARBALL_DIR/"
    cp -r configs "$TARBALL_DIR/"
    cp -r scripts "$TARBALL_DIR/"
    cp -r systemd "$TARBALL_DIR/"
    cp -r python-service "$TARBALL_DIR/"
    # Note: frontend is embedded in linuxhello binary
    cp README.md Makefile "$TARBALL_DIR/"
    
    # Create install script
    cat > "$TARBALL_DIR/install.sh" << 'EOF'
#!/bin/bash
# LinuxHello installation script

echo "Installing LinuxHello..."

# Check if running as root
if [ "$(id -u)" != "0" ]; then
   echo "This script must be run as root" 
   exit 1
fi

# Install files
install -d /usr/local/bin
install -d /usr/local/lib/security
install -d /etc/linuxhello
install -d /opt/linuxhello
install -d /var/lib/linuxhello

install -m 755 bin/linuxhello /usr/local/bin/
install -m 755 bin/pam_linuxhello.so /usr/local/lib/security/ 2>/dev/null || true
install -m 644 configs/linuxhello.conf /etc/linuxhello/
cp -r python-service /opt/linuxhello/
cp systemd/linuxhello-inference.service /etc/systemd/system/

# Set up Python environment
cd /opt/linuxhello/python-service
python3 -m venv venv
./venv/bin/pip install --upgrade pip
./venv/bin/pip install -r requirements.txt

systemctl daemon-reload
echo "Installation complete!"
echo "Start inference: systemctl enable --now linuxhello-inference"
echo "Launch GUI: sudo linuxhello"
EOF
    chmod +x "$TARBALL_DIR/install.sh"
    
    # Create tarball
    (cd "$BUILD_DIR" && tar -czf "$PACKAGE_DIR/linuxhello-${VERSION}-linux-${ARCH}.tar.gz" linuxhello-generic)
    echo "Generic tarball created: linuxhello-${VERSION}-linux-${ARCH}.tar.gz"
fi

echo "Package build complete! Packages are in: $PACKAGE_DIR"
ls -la "$PACKAGE_DIR"