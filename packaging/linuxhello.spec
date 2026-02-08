%global debug_package %{nil}

Name:           linuxhello
Version:        1.3.4
Release:        1%{?dist}
Summary:        Face authentication system for Linux
License:        MIT
URL:            https://github.com/MrCodeEU/LinuxHello
Source0:        %{name}-%{version}.tar.gz

BuildRequires:  golang >= 1.24
BuildRequires:  nodejs >= 18
BuildRequires:  npm
BuildRequires:  libv4l-devel
BuildRequires:  pam-devel
BuildRequires:  sqlite-devel
BuildRequires:  systemd-rpm-macros

Requires:       pam
Requires:       sqlite
Requires:       python3
Requires:       python3-pip
Requires:       systemd
Requires:       polkit
Requires:       shadow-utils

%description
LinuxHello is a modern face authentication system for Linux that integrates
with PAM (Pluggable Authentication Modules) to provide secure, touchless
authentication for system login, sudo, and other services.

Features:
- Face detection and recognition using AI models
- PAM integration for system authentication
- Desktop management interface
- Multi-user enrollment support
- Liveness detection to prevent photo/video spoofing

%prep
%setup -q

%build
# Build web frontend (embedded into Wails GUI binary)
cd frontend
npm ci
npm run build
cd ..

# Build Go binaries
export CGO_ENABLED=1
go mod download

# Build single binary (Wails app with all subcommands, embeds frontend/dist)
go build -ldflags="-s -w" -tags desktop,production -o bin/linuxhello .

# Build PAM module
CGO_ENABLED=1 go build -buildmode=c-shared -ldflags="-s -w" -o bin/pam_linuxhello.so ./pkg/pam

%install
# Create directories
install -d %{buildroot}%{_bindir}
install -d %{buildroot}%{_libdir}/security
install -d %{buildroot}%{_sysconfdir}/linuxhello
install -d %{buildroot}%{_datadir}/linuxhello
install -d %{buildroot}%{_datadir}/linuxhello/python-service
install -d %{buildroot}%{_datadir}/linuxhello/models
install -d %{buildroot}%{_datadir}/linuxhello/icons
install -d %{buildroot}%{_datadir}/applications
install -d %{buildroot}%{_datadir}/icons/hicolor/scalable/apps
install -d %{buildroot}%{_unitdir}
install -d %{buildroot}%{_localstatedir}/lib/linuxhello
install -d %{buildroot}%{_localstatedir}/log

# Install binaries
install -m 755 bin/linuxhello %{buildroot}%{_bindir}/
install -m 755 bin/pam_linuxhello.so %{buildroot}%{_libdir}/security/
install -m 755 scripts/linuxhello-pam %{buildroot}%{_bindir}/

# Install configuration
install -m 644 configs/linuxhello.conf %{buildroot}%{_sysconfdir}/linuxhello/

# Note: frontend is embedded in linuxhello binary, no separate install needed

# Install Python service
cp python-service/*.py %{buildroot}%{_datadir}/linuxhello/python-service/
cp python-service/requirements.txt %{buildroot}%{_datadir}/linuxhello/python-service/

# Install models (if present)
cp models/arcface_r50.onnx %{buildroot}%{_datadir}/linuxhello/models/ 2>/dev/null || true
cp models/det_10g.onnx %{buildroot}%{_datadir}/linuxhello/models/ 2>/dev/null || true

# Install icons
for size in 16 24 32 48 64 128 256 512; do
    if [ -f assets/linuxhello-icon-${size}.png ]; then
        install -d %{buildroot}%{_datadir}/icons/hicolor/${size}x${size}/apps
        install -m 644 assets/linuxhello-icon-${size}.png %{buildroot}%{_datadir}/icons/hicolor/${size}x${size}/apps/linuxhello.png
    fi
done
install -m 644 assets/linuxhello-icon.svg %{buildroot}%{_datadir}/icons/hicolor/scalable/apps/linuxhello.svg
install -m 644 assets/linuxhello-icon-*.png %{buildroot}%{_datadir}/linuxhello/icons/ 2>/dev/null || true
install -m 644 assets/linuxhello-icon.svg %{buildroot}%{_datadir}/linuxhello/icons/ 2>/dev/null || true

# Install systemd service (inference only; GUI is a desktop app)
install -m 644 systemd/linuxhello-inference.service %{buildroot}%{_unitdir}/

# Install desktop launcher
install -m 644 packaging/linuxhello.desktop %{buildroot}%{_datadir}/applications/

%pre
# Create linuxhello user and group
getent group linuxhello >/dev/null || groupadd -r linuxhello
getent passwd linuxhello >/dev/null || \
    useradd -r -g linuxhello -d %{_localstatedir}/lib/linuxhello -s /sbin/nologin \
    -c "LinuxHello Face Authentication" linuxhello

%post
# Set permissions
chown -R linuxhello:linuxhello %{_localstatedir}/lib/linuxhello
chmod 755 %{_localstatedir}/lib/linuxhello

# Enable and start inference service
%systemd_post linuxhello-inference.service

echo "Starting LinuxHello inference service..."
if systemctl start linuxhello-inference.service 2>/dev/null; then
    echo "Inference service started"
else
    echo "Inference service failed to start (may need manual setup)"
fi

echo ""
echo "LinuxHello installed successfully!"
echo ""
echo "Next steps:"
echo "   1. Launch the desktop GUI: sudo linuxhello"
echo "   2. Enroll your face: sudo linuxhello enroll -user \$USER"
echo "   3. Enable PAM auth: sudo linuxhello-pam enable sudo"
echo ""

%preun
%systemd_preun linuxhello-inference.service

%postun
%systemd_postun_with_restart linuxhello-inference.service

%files
%license LICENSE
%doc README.md
%{_bindir}/linuxhello
%{_bindir}/linuxhello-pam
%{_libdir}/security/pam_linuxhello.so
%config(noreplace) %{_sysconfdir}/linuxhello/linuxhello.conf
%{_datadir}/linuxhello/
%{_datadir}/applications/linuxhello.desktop
%{_datadir}/icons/hicolor/*/apps/linuxhello.*
%{_unitdir}/linuxhello-inference.service
%dir %{_localstatedir}/lib/linuxhello

%changelog
* Thu Feb 06 2026 MrCode <mrcode@example.com> - 1.3.4-3
- Consolidated all binaries into single linuxhello binary with subcommands
- Removed linuxhello-enroll, linuxhello-test, linuxhello-gui separate binaries
- GUI, daemon, enroll, and test all accessible via linuxhello subcommands

* Thu Feb 06 2025 MrCode <mrcode@example.com> - 1.3.4-2
- Migrated GUI from HTTP server to Wails v2 desktop application
- Frontend now embedded in linuxhello-gui binary
- Removed GUI systemd service (desktop app requires display server)
- Updated build to compile Wails app from project root

* Mon Feb 04 2024 MrCode <mrcode@example.com> - 0.1.1-1
- Initial RPM package
- Face authentication with PAM integration
- Web management interface
- Multi-user support