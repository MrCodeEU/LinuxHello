Name:           linuxhello
Version:        1.0.0
Release:        1%{?dist}
Summary:        Face authentication system for Linux with PAM integration
License:        MIT
URL:            https://github.com/MrCodeEU/LinuxHello
Source0:        %{name}-%{version}.tar.gz

BuildRequires:  golang >= 1.24
BuildRequires:  gcc
BuildRequires:  gcc-c++
BuildRequires:  make
BuildRequires:  libv4l-devel
BuildRequires:  pam-devel
BuildRequires:  sqlite-devel
BuildRequires:  pkg-config
BuildRequires:  nodejs >= 18
BuildRequires:  npm
BuildRequires:  systemd-rpm-macros

Requires:       pam
Requires:       sqlite
Requires:       libv4l
Requires:       python3
Requires:       python3-pip
Requires:       python3-venv

%description
LinuxHello is a modern face authentication system for Linux that integrates
with PAM (Pluggable Authentication Modules) to provide secure, contactless
authentication. It includes a desktop management interface, face enrollment
tools, and comprehensive authentication testing capabilities.

Features:
- Face detection and recognition using ONNX models
- PAM integration for system authentication (sudo, login, etc.)
- Desktop GUI for management and configuration
- Real-time liveness detection
- Multi-user enrollment support
- Comprehensive logging and audit trail

%prep
%autosetup -n %{name}-%{version}

%build
# Build web frontend (embedded into Wails GUI binary)
cd frontend
npm ci --omit=dev
npm run build
cd ..

# Build Go binaries
export CGO_ENABLED=1
# Build single binary (Wails app with all subcommands, embeds frontend/dist)
%gobuild -tags desktop,production -o bin/linuxhello .

# Build PAM module
CGO_ENABLED=1 CGO_CFLAGS="-I/usr/include" CGO_LDFLAGS="-lpam -lpam_misc" \
go build -buildmode=c-shared -o bin/pam_linuxhello.so ./pkg/pam

%install
# Create directories
install -d %{buildroot}%{_bindir}
install -d %{buildroot}%{_libdir}/security
install -d %{buildroot}%{_sysconfdir}/linuxhello
install -d %{buildroot}%{_datadir}/linuxhello/python-service
install -d %{buildroot}%{_datadir}/linuxhello/models
install -d %{buildroot}%{_datadir}/linuxhello/icons
install -d %{buildroot}%{_localstatedir}/lib/linuxhello
install -d %{buildroot}%{_unitdir}
install -d %{buildroot}%{_localstatedir}/log
install -d %{buildroot}%{_datadir}/applications

# Install binaries
install -m 755 bin/linuxhello %{buildroot}%{_bindir}/
install -m 755 scripts/linuxhello-pam %{buildroot}%{_bindir}/

# Install PAM module
install -m 755 bin/pam_linuxhello.so %{buildroot}%{_libdir}/security/

# Install configuration
install -m 644 configs/linuxhello.conf %{buildroot}%{_sysconfdir}/linuxhello/

# Install Python service
cp -r python-service/*.py %{buildroot}%{_datadir}/linuxhello/python-service/
cp python-service/requirements.txt %{buildroot}%{_datadir}/linuxhello/python-service/

# Note: frontend is embedded in linuxhello binary, no separate install needed

# Install icons
install -m 644 assets/linuxhello-icon-*.png %{buildroot}%{_datadir}/linuxhello/icons/ 2>/dev/null || true
install -m 644 assets/favicon.ico %{buildroot}%{_datadir}/linuxhello/icons/ 2>/dev/null || true

# Install desktop entry
install -m 644 packaging/linuxhello.desktop %{buildroot}%{_datadir}/applications/

# Install systemd service (inference only; GUI is a desktop app)
install -m 644 systemd/linuxhello-inference.service %{buildroot}%{_unitdir}/

# Install models directory (empty, will be populated post-install)
touch %{buildroot}%{_datadir}/linuxhello/models/README.md
echo "Place ONNX models (arcface_r50.onnx, det_10g.onnx) in this directory" > %{buildroot}%{_datadir}/linuxhello/models/README.md

%pre
# Create linuxhello user for the service
getent group linuxhello >/dev/null || groupadd -r linuxhello
getent passwd linuxhello >/dev/null || \
    useradd -r -g linuxhello -d %{_localstatedir}/lib/linuxhello -s /sbin/nologin \
    -c "LinuxHello face authentication service" linuxhello

%post
%systemd_post linuxhello-inference.service

# Set up Python virtual environment
if [ "$1" = 1 ]; then
    cd %{_datadir}/linuxhello/python-service
    python3 -m venv venv
    ./venv/bin/pip install --upgrade pip
    ./venv/bin/pip install -r requirements.txt
fi

# Set permissions
chown -R linuxhello:linuxhello %{_localstatedir}/lib/linuxhello
chmod 755 %{_libdir}/security/pam_linuxhello.so

# Create log file
touch %{_localstatedir}/log/linuxhello.log
chown linuxhello:linuxhello %{_localstatedir}/log/linuxhello.log
chmod 644 %{_localstatedir}/log/linuxhello.log

echo "LinuxHello installed successfully!"
echo ""
echo "Next steps:"
echo "1. Download ONNX models to %{_datadir}/linuxhello/models/"
echo "   - arcface_r50.onnx (face recognition)"
echo "   - det_10g.onnx (SCRFD face detection)"
echo "2. Start the inference service: systemctl enable --now linuxhello-inference"
echo "3. Launch the desktop GUI: sudo linuxhello"
echo "4. Use the GUI to enroll faces and enable PAM integration"

%preun
%systemd_preun linuxhello-inference.service

%postun
%systemd_postun_with_restart linuxhello-inference.service

if [ "$1" = 0 ]; then
    # Remove user and group on uninstall
    userdel linuxhello >/dev/null 2>&1 || :
    groupdel linuxhello >/dev/null 2>&1 || :
fi

%files
%license LICENSE
%doc README.md
%{_bindir}/linuxhello
%{_bindir}/linuxhello-pam
%{_libdir}/security/pam_linuxhello.so
%config(noreplace) %{_sysconfdir}/linuxhello/linuxhello.conf
%{_datadir}/linuxhello/
%{_datadir}/applications/linuxhello.desktop
%{_unitdir}/linuxhello-inference.service
%attr(755,linuxhello,linuxhello) %dir %{_localstatedir}/lib/linuxhello

%changelog
* Thu Feb 06 2026 MrCode <mrcode@example.com> - 1.0.0-3
- Consolidated all binaries into single linuxhello binary with subcommands
- Removed linuxhello-enroll, linuxhello-test, linuxhello-gui separate binaries
- GUI, daemon, enroll, and test all accessible via linuxhello subcommands

* Thu Feb 06 2025 MrCode <mrcode@example.com> - 1.0.0-2
- Migrated GUI from HTTP server to Wails v2 desktop application
- Frontend now embedded in linuxhello-gui binary
- Removed GUI systemd service (desktop app requires display server)
- Updated build to compile Wails app from project root

* Thu Feb 04 2024 MrCode <mrcode@example.com> - 1.0.0-1
- Initial RPM package
- Added web-based GUI for management
- Added authentication test page with visual debugging
- Fixed PAM integration for Fedora/RHEL systems
- Added comprehensive systemd service integration
- Added automatic Python virtual environment setup