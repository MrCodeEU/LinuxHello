%global debug_package %{nil}

Name:           linuxhello
Version:        1.3.4
Release:        1%{?dist}
Summary:        Face authentication system for Linux
License:        MIT
URL:            https://github.com/MrCodeEU/LinuxHello
Source0:        %{name}-%{version}.tar.gz

BuildRequires:  golang >= 1.19
BuildRequires:  nodejs >= 16
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
- Web-based management interface
- Multi-user enrollment support
- Liveness detection to prevent photo/video spoofing

%prep
%setup -q

%build
# Build web frontend
cd web-ui
npm ci
npm run build
cd ..

# Build Go binaries
export CGO_ENABLED=1
go mod download
make build

%install
# Create directories
install -d %{buildroot}%{_bindir}
install -d %{buildroot}%{_libdir}/security
install -d %{buildroot}%{_sysconfdir}/linuxhello
install -d %{buildroot}%{_datadir}/linuxhello
install -d %{buildroot}%{_datadir}/linuxhello/web-ui
install -d %{buildroot}%{_datadir}/linuxhello/python-service
install -d %{buildroot}%{_datadir}/linuxhello/models
install -d %{buildroot}%{_datadir}/applications
install -d %{buildroot}%{_unitdir}
install -d %{buildroot}%{_localstatedir}/lib/linuxhello
install -d %{buildroot}%{_localstatedir}/log

# Install binaries
install -m 755 bin/linuxhello %{buildroot}%{_bindir}/
install -m 755 bin/linuxhello-enroll %{buildroot}%{_bindir}/
install -m 755 bin/linuxhello-test %{buildroot}%{_bindir}/
install -m 755 bin/linuxhello-gui %{buildroot}%{_bindir}/
install -m 755 bin/pam_linuxhello.so %{buildroot}%{_libdir}/security/
install -m 755 scripts/linuxhello-pam %{buildroot}%{_bindir}/

# Install configuration
install -m 644 configs/linuxhello.conf %{buildroot}%{_sysconfdir}/linuxhello/

# Install web interface
cp -r web-ui/dist/* %{buildroot}%{_datadir}/linuxhello/web-ui/

# Install Python service
cp python-service/*.py %{buildroot}%{_datadir}/linuxhello/python-service/
cp python-service/requirements.txt %{buildroot}%{_datadir}/linuxhello/python-service/

# Install models
cp models/README.md %{buildroot}%{_datadir}/linuxhello/models/
cp models/arcface_r50.onnx %{buildroot}%{_datadir}/linuxhello/models/
cp models/scrfd_person_2.5g.onnx %{buildroot}%{_datadir}/linuxhello/models/

# Install systemd services
install -m 644 systemd/linuxhello-inference.service %{buildroot}%{_unitdir}/
install -m 644 packaging/linuxhello-gui.service %{buildroot}%{_unitdir}/

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

# Enable and start services
%systemd_post linuxhello-inference.service
%systemd_post linuxhello-gui.service

# Start services automatically (with error handling)
echo "Starting LinuxHello services..."
if systemctl start linuxhello-inference.service 2>/dev/null; then
    echo "✅ Inference service started"
else
    echo "⚠️  Inference service failed to start (may need manual setup)"
fi

if systemctl start linuxhello-gui.service 2>/dev/null; then
    echo "✅ GUI service started"
else
    echo "⚠️  GUI service failed to start (check dependencies)"
fi

# Check if services are running
sleep 2
if systemctl is-active --quiet linuxhello-inference.service && systemctl is-active --quiet linuxhello-gui.service; then
    echo ""
    echo "🎉 LinuxHello installed and started successfully!"
    echo ""
    echo "🚀 Ready to use:"
    echo "   • Web interface: http://localhost:8080"
    echo "   • Management GUI: linuxhello-gui"
    echo "   • Enrollment tool: linuxhello-enroll"
    echo ""
    echo "📋 Next steps:"
    echo "   1. Open GUI: http://localhost:8080"
    echo "   2. Enroll your face: linuxhello-enroll"
    echo "   3. Enable PAM auth: linuxhello-pam enable-sudo"
    echo ""
else
    echo ""
    echo "⚠️  LinuxHello installed but some services may need manual attention."
    echo ""
    echo "🔧 Manual startup:"
    echo "   sudo systemctl start linuxhello-inference linuxhello-gui"
    echo ""
    echo "📋 Next steps:"
    echo "   1. Check logs: sudo journalctl -u linuxhello-inference -u linuxhello-gui"
    echo "   2. Open GUI: http://localhost:8080"
    echo "   3. Enroll users and enable PAM authentication"
fi
echo ""

%preun
%systemd_preun linuxhello-inference.service
%systemd_preun linuxhello-gui.service

%postun
%systemd_postun_with_restart linuxhello-inference.service
%systemd_postun_with_restart linuxhello-gui.service

%files
%license LICENSE
%doc README.md
%{_bindir}/linuxhello
%{_bindir}/linuxhello-enroll
%{_bindir}/linuxhello-test
%{_bindir}/linuxhello-gui
%{_bindir}/linuxhello-pam
%{_libdir}/security/pam_linuxhello.so
%config(noreplace) %{_sysconfdir}/linuxhello/linuxhello.conf
%{_datadir}/linuxhello/
%{_datadir}/applications/linuxhello.desktop
%{_unitdir}/linuxhello-inference.service
%{_unitdir}/linuxhello-gui.service
%dir %{_localstatedir}/lib/linuxhello

%changelog
* Mon Feb 04 2024 MrCode <mrcode@example.com> - 0.1.1-1
- Initial RPM package
- Face authentication with PAM integration
- Web management interface
- Multi-user support