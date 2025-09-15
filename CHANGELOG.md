# Changelog

All notable changes to this project will be documented in this file.

The format is based on [Keep a Changelog](https://keepachangelog.com/en/1.0.0/),
and this project adheres to [Semantic Versioning](https://semver.org/spec/v2.0.0.html).

## [Unreleased]

### Planned
- Live migration between QNAP hosts (Phase 2)
- VM console access (VNC/serial) (Phase 2)
- VM templates and marketplace (Phase 3)

## [0.2.0] - TBD

### Added
- **VM Snapshots**: Complete snapshot lifecycle management (create, list, restore, delete, current)
- **Resource Monitoring**: Real-time VM statistics (CPU, memory, disk I/O, network)
- **VM Cloning**: Full and linked VM cloning capabilities
- **Watch Mode**: Real-time statistics monitoring with customizable intervals
- **Enhanced Integration Tests**: Comprehensive testing of Phase 2 features against real hardware

### Improved
- **Disk Format Support**: Proper qcow2 driver specification for snapshot compatibility
- **Error Handling**: Enhanced error messages and validation throughout
- **Documentation**: Updated roadmap, commands, and usage examples
- **CLI Interface**: Rich help system for all new commands and options

## [0.1.0] - 2024-09-15

### Added
- **Core VM Operations**: Complete VM lifecycle management (create, start, stop, delete, list, status)
- **SSH Integration**: Secure connection handling with SSH key authentication and ssh-agent support
- **libvirt/virsh Support**: Native integration with QNAP Virtualization Station via libvirt
- **Configuration Management**: YAML-based configuration files with multi-host support
- **Intelligent Storage Detection**: Automatic detection and management of QNAP storage pools (CACHEDEV, ZFS, USB)
- **Cross-platform Support**: Binaries for Linux, macOS, and Windows
- **Comprehensive CLI**: Rich command-line interface with detailed help and error handling
- **Professional Tooling**: Pre-commit hooks, CI/CD workflows, and comprehensive test suite