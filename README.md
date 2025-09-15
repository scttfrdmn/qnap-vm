# qnap-vm

A command-line tool for managing virtual machines on QNAP devices with Virtualization Station.

## Features

- **VM Lifecycle Management**: Create, start, stop, and delete virtual machines
- **SSH-based Connection**: Secure remote management of QNAP devices
- **libvirt Integration**: Built on proven virtualization technologies
- **Template Support**: Pre-configured VM templates for common use cases
- **Storage Management**: Intelligent detection and management of QNAP storage pools
- **Cross-platform**: Works on macOS, Linux, and Windows

## Installation

### Homebrew (macOS/Linux)

```bash
brew install scttfrdmn/qnap-vm/qnap-vm
```

### From Release

Download the latest release from [GitHub Releases](https://github.com/scttfrdmn/qnap-vm/releases).

### From Source

```bash
go install github.com/scttfrdmn/qnap-vm/cmd/qnap-vm@latest
```

## Quick Start

1. Configure your QNAP connection:
   ```bash
   qnap-vm config set --host your-qnap.local --username admin
   ```

2. List existing VMs:
   ```bash
   qnap-vm list
   ```

3. Create a new VM:
   ```bash
   qnap-vm create --name my-vm --template ubuntu-20.04
   ```

4. Start the VM:
   ```bash
   qnap-vm start my-vm
   ```

## System Requirements

### QNAP Device Requirements
- QNAP NAS with Virtualization Station installed
- QTS 5.1.0+ or QuTS hero h5.1.0+
- CPU with Intel VT-x or AMD-V support
- Minimum 4GB RAM (varies by NAS model)

### Client Requirements
- SSH access to QNAP device
- Network connectivity to QNAP device

## Testing

qnap-vm follows the same rigorous testing approach as qnap-docker, with comprehensive testing against real QNAP hardware.

### Unit Tests

Run standard unit tests:
```bash
make test
make test-coverage  # with coverage report
```

### Integration Tests

**Real hardware testing** against actual QNAP devices:

```bash
# Required: Set your QNAP device details
export NAS_HOST=your-qnap.local
export NAS_USER=admin  # optional, defaults to 'admin'

# Run integration tests
make integration-test

# Run with coverage reporting
make integration-test-full
```

**Integration test coverage:**
- ✅ SSH connection and authentication
- ✅ Virtualization Station availability
- ✅ Storage pool detection and management
- ✅ Complete VM lifecycle (create, start, stop, delete)
- ✅ VM configuration validation
- ✅ Resource management and cleanup

**Requirements:**
- QNAP NAS with Virtualization Station installed
- QTS 5.1.0+ or QuTS hero h5.1.0+
- SSH access enabled
- Intel VT-x or AMD-V CPU support

See [tests/integration/README.md](tests/integration/README.md) for detailed integration testing documentation.

## Development Roadmap

### Phase 1: Core VM Lifecycle (v0.1.0)
- [x] Project setup and basic structure
- [ ] SSH connection management
- [ ] libvirt/virsh integration
- [ ] Basic VM operations (create, start, stop, delete, list)
- [ ] Configuration file support
- [ ] Initial CLI commands

### Phase 2: Advanced Features (v0.2.0)
- [ ] VM snapshots and restoration
- [ ] VM cloning capabilities
- [ ] Live migration between hosts
- [ ] Resource monitoring and statistics
- [ ] VM console access

### Phase 3: Templates and Automation (v0.3.0)
- [ ] VM template system
- [ ] Automated VM provisioning
- [ ] Bulk operations
- [ ] Export/import VM configurations
- [ ] Integration with cloud-init

### Phase 4: Storage and Networking (v0.4.0)
- [ ] Advanced storage pool management
- [ ] Network configuration management
- [ ] Virtual disk operations
- [ ] Storage migration
- [ ] Network isolation and VLANs

## Configuration

qnap-vm uses YAML configuration files stored in `~/.qnap-vm/config.yaml`.

Example configuration:
```yaml
hosts:
  default:
    host: qnap.local
    username: admin
    port: 22
    keyfile: ~/.ssh/id_rsa
```

## Commands

| Command | Description |
|---------|-------------|
| `qnap-vm list` | List all virtual machines |
| `qnap-vm create` | Create a new virtual machine |
| `qnap-vm start` | Start a virtual machine |
| `qnap-vm stop` | Stop a virtual machine |
| `qnap-vm delete` | Delete a virtual machine |
| `qnap-vm status` | Show VM status and resource usage |
| `qnap-vm config` | Manage connection configuration |

## Contributing

Contributions are welcome! Please read our [Contributing Guidelines](CONTRIBUTING.md) and [Code of Conduct](CODE_OF_CONDUCT.md).

## License

This project is licensed under the MIT License - see the [LICENSE](LICENSE) file for details.

## Acknowledgments

- Inspired by [qnap-docker](https://github.com/scttfrdmn/qnap-docker)
- Built on libvirt and QEMU technologies
- Thanks to the QNAP community for virtualization insights