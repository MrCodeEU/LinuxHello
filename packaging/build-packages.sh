#!/bin/bash

set -euo pipefail

SCRIPT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
ROOT_DIR="$(cd "$SCRIPT_DIR/.." && pwd)"
SPEC_FILE="$ROOT_DIR/packaging/linuxhello.spec"

VERSION="${1:-$(grep '^Version:' "$SPEC_FILE" | sed 's/Version:[[:space:]]*//')}"
ARCH="${2:-$(uname -m)}"
BUILD_DIR="/tmp/linuxhello-build"
PACKAGE_DIR="$ROOT_DIR/dist/packages"

map_deb_arch() {
    case "$1" in
        x86_64|amd64) echo "amd64" ;;
        aarch64|arm64) echo "arm64" ;;
        armv7l) echo "armhf" ;;
        i686|i386) echo "i386" ;;
        *) echo "$1" ;;
    esac
}

echo "Building LinuxHello packages for version $VERSION on $ARCH"

cd "$ROOT_DIR"
rm -rf "$BUILD_DIR" "$PACKAGE_DIR"
mkdir -p "$BUILD_DIR" "$PACKAGE_DIR"

if [ ! -x "$ROOT_DIR/bin/linuxhello" ] || [ ! -x "$ROOT_DIR/bin/pam_linuxhello.so" ]; then
    echo "Missing build artifacts, running make build..."
    make build
fi

echo "Creating source tarball..."
tar \
    --exclude=".git" \
    --exclude="dist" \
    --exclude="node_modules" \
    --exclude="frontend/node_modules" \
    --exclude="/tmp/linuxhello-build" \
    -czf "$BUILD_DIR/linuxhello-$VERSION.tar.gz" \
    --transform "s|^|linuxhello-$VERSION/|" \
    .

if command -v rpmbuild >/dev/null 2>&1; then
    echo "Building RPM package..."

    RPMBUILD_DIR="$BUILD_DIR/rpmbuild"
    mkdir -p "$RPMBUILD_DIR"/{BUILD,RPMS,SOURCES,SPECS,SRPMS}

    cp "$BUILD_DIR/linuxhello-$VERSION.tar.gz" "$RPMBUILD_DIR/SOURCES/"
    cp "$SPEC_FILE" "$RPMBUILD_DIR/SPECS/linuxhello.spec"

    sed -i "s/^Version:.*/Version:        $VERSION/" "$RPMBUILD_DIR/SPECS/linuxhello.spec"

    rpmbuild --define "_topdir $RPMBUILD_DIR" -ba "$RPMBUILD_DIR/SPECS/linuxhello.spec"

    find "$RPMBUILD_DIR/RPMS" -name "*.rpm" -exec cp {} "$PACKAGE_DIR/" \;
    find "$RPMBUILD_DIR/SRPMS" -name "*.rpm" -exec cp {} "$PACKAGE_DIR/" \;
else
    echo "rpmbuild not found, skipping RPM build"
fi

if command -v dpkg-deb >/dev/null 2>&1; then
    echo "Building DEB package..."

    DEB_ARCH="$(map_deb_arch "$ARCH")"
    DEB_DIR="$BUILD_DIR/linuxhello-deb"
    mkdir -p "$DEB_DIR/DEBIAN"

    make install DESTDIR="$DEB_DIR" PREFIX=/usr

    cat > "$DEB_DIR/DEBIAN/control" << EOF
Package: linuxhello
Version: $VERSION
Section: admin
Priority: optional
Architecture: $DEB_ARCH
Depends: libpam-modules, libsqlite3-0, python3, python3-venv, python3-pip, systemd
Maintainer: MrCode <mrcode@example.com>
Description: Face authentication system for Linux
 LinuxHello integrates facial recognition with PAM for secure authentication.
EOF

    cat > "$DEB_DIR/DEBIAN/postinst" << 'EOF'
#!/bin/bash
set -e

if ! getent group linuxhello >/dev/null; then
    groupadd --system linuxhello || true
fi
if ! getent passwd linuxhello >/dev/null; then
    useradd --system --gid linuxhello --home /var/lib/linuxhello --shell /usr/sbin/nologin linuxhello || true
fi

mkdir -p /var/lib/linuxhello
chown -R linuxhello:linuxhello /var/lib/linuxhello /usr/share/linuxhello/python-service
chmod 750 /var/lib/linuxhello

/usr/libexec/linuxhello/sync-python-venv.sh /usr/share/linuxhello/python-service || true

systemctl daemon-reload || true
systemctl enable --now linuxhello-inference.service || true
EOF

    cat > "$DEB_DIR/DEBIAN/prerm" << 'EOF'
#!/bin/bash
set -e

if [ "$1" = "remove" ]; then
    systemctl disable --now linuxhello-inference.service || true
fi
EOF

    chmod 755 "$DEB_DIR/DEBIAN/postinst" "$DEB_DIR/DEBIAN/prerm"

    if dpkg-deb --help | grep -q -- '--root-owner-group'; then
        dpkg-deb --root-owner-group --build "$DEB_DIR" "$PACKAGE_DIR/linuxhello_${VERSION}_${DEB_ARCH}.deb"
    else
        dpkg-deb --build "$DEB_DIR" "$PACKAGE_DIR/linuxhello_${VERSION}_${DEB_ARCH}.deb"
    fi

    echo "DEB package created: linuxhello_${VERSION}_${DEB_ARCH}.deb"
else
    echo "dpkg-deb not found, skipping DEB build"
fi

echo "Creating generic tarball..."
TARBALL_ROOT="$BUILD_DIR/linuxhello-generic"
TARBALL_STAGE="$TARBALL_ROOT/payload"
mkdir -p "$TARBALL_STAGE"

make install DESTDIR="$TARBALL_STAGE" PREFIX=/usr

cat > "$TARBALL_ROOT/install.sh" << 'EOF'
#!/bin/bash
set -euo pipefail

if [ "$(id -u)" -ne 0 ]; then
    echo "This installer must be run as root" >&2
    exit 1
fi

SCRIPT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
rsync -a "$SCRIPT_DIR/payload/" /

if ! getent group linuxhello >/dev/null; then
    groupadd --system linuxhello || true
fi
if ! getent passwd linuxhello >/dev/null; then
    useradd --system --gid linuxhello --home /var/lib/linuxhello --shell /usr/sbin/nologin linuxhello || true
fi

mkdir -p /var/lib/linuxhello
chown -R linuxhello:linuxhello /var/lib/linuxhello /usr/share/linuxhello/python-service
chmod 750 /var/lib/linuxhello

/usr/libexec/linuxhello/sync-python-venv.sh /usr/share/linuxhello/python-service

systemctl daemon-reload
systemctl enable --now linuxhello-inference.service

echo "Installation complete"
echo "Launch GUI: sudo linuxhello"
EOF

chmod +x "$TARBALL_ROOT/install.sh"
(cd "$BUILD_DIR" && tar -czf "$PACKAGE_DIR/linuxhello-${VERSION}-linux-${ARCH}.tar.gz" linuxhello-generic)

echo "Package build complete. Artifacts in: $PACKAGE_DIR"
ls -la "$PACKAGE_DIR"