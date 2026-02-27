# Changelog

All notable changes to this project will be documented in this file.

The format is based on [Keep a Changelog](https://keepachangelog.com/en/1.0.0/),
and this project adheres to [Semantic Versioning](https://semver.org/spec/v2.0.0.html).

## [1.7.0] - 2026-02-27

### Added
- **Windows Hello-like authentication implementation**: Full facial recognition authentication system
- **Real-time status feedback during authentication**: Visual feedback during face capture and verification
- **Configurable liveness failure handling**: Configurable number of allowed liveness failures before fallback
- **Graceful fallback to password authentication**: Automatic fallback to password when face authentication fails
- **New configuration options**:
  - `MaxLivenessFailures`: Maximum allowed liveness check failures
  - `MaxFaceAuthAttempts`: Maximum face authentication attempts
  - `ShowStatusMessages`: Toggle status message display during authentication

### Changed
- Enhanced authentication engine with improved face recognition accuracy
- Updated PAM module to support new authentication flow
- Improved camera handling for better performance and reliability
- Enhanced error handling and user feedback mechanisms

### Fixed
- Resolved camera initialization issues on certain hardware
- Fixed memory leaks in authentication engine
- Corrected PAM configuration file permissions
- Resolved race conditions in concurrent authentication attempts

### Security
- Implemented enhanced liveness detection to prevent spoofing attacks
- Added secure credential storage for authentication data
- Improved session management and timeout handling

## [1.6.0] - 2026-02-15

### Added
- Initial Windows Hello-like authentication implementation
- Basic facial recognition capabilities
- PAM module integration
- Web-based administration interface

### Changed
- Refactored authentication engine architecture
- Improved camera integration
- Enhanced configuration management

### Fixed
- Various bug fixes and stability improvements

## [1.5.0] - 2026-02-01

### Added
- Initial release with basic authentication functionality
- PAM module support
- Configuration management
- Basic web interface

---

[Unreleased]: https://github.com/mrcodeeu/LinuxHello/compare/v1.7.0...HEAD
[1.7.0]: https://github.com/mrcodeeu/LinuxHello/compare/v1.6.0...v1.7.0
[1.6.0]: https://github.com/mrcodeeu/LinuxHello/compare/v1.5.0...v1.6.0
[1.5.0]: https://github.com/mrcodeeu/LinuxHello/compare/v1.4.0...v1.5.0