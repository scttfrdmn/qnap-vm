# Changelog

All notable changes to this project will be documented in this file.

The format is based on [Keep a Changelog](https://keepachangelog.com/en/1.0.0/),
and this project adheres to [Semantic Versioning](https://semver.org/spec/v2.0.0.html).

## [Unreleased]

### Planned
- VM snapshots and restoration (Phase 2)
- VM cloning capabilities (Phase 2)
- Live migration between hosts (Phase 2)

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