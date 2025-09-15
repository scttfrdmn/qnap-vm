# Changelog

All notable changes to this project will be documented in this file.

The format is based on [Keep a Changelog](https://keepachangelog.com/en/1.0.0/),
and this project adheres to [Semantic Versioning](https://semver.org/spec/v2.0.0.html).

## [Unreleased]

### Planned for Phase 3 (v0.3.0)
- Bulk VM operations (start/stop/delete multiple VMs)
- VM configuration export/import (backup and restore)
- Automated VM provisioning with scripts
- Scheduled operations and VM lifecycle automation
- Multi-VM management and orchestration

### Planned for Phase 4 (v0.4.0)
- Live migration between QNAP hosts
- Advanced storage pool management
- Network configuration with VLAN support

## [0.2.0] - 2024-09-15

### Added
- **VM Snapshots**: Complete snapshot lifecycle management (create, list, restore, delete, current)
- **Resource Monitoring**: Real-time VM statistics (CPU, memory, disk I/O, network)
- **VM Cloning**: Full and linked VM cloning capabilities with automatic configuration inheritance
- **Console Access**: VNC and serial console support with connection guidance and SSH tunneling
- **Watch Mode**: Real-time statistics monitoring with customizable refresh intervals
- **Enhanced Integration Tests**: Comprehensive testing of all Phase 2 features against real hardware

### Improved
- **Disk Format Support**: Proper qcow2 driver specification for snapshot compatibility
- **Error Handling**: Enhanced error messages and validation throughout all operations
- **Documentation**: Updated roadmap, comprehensive command examples, and usage guides
- **CLI Interface**: Rich help system for all new commands with detailed options
- **Project Structure**: Reorganized to match qnap-docker professional standards

### Technical Enhancements
- **libvirt Integration**: Advanced virsh commands for snapshots, stats, and console access
- **XML Configuration**: Enhanced domain XML with proper driver and emulator specifications
- **Resource Parsing**: Sophisticated statistics parsing for comprehensive monitoring
- **Console Detection**: Intelligent VNC and serial console discovery and configuration

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