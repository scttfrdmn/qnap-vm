# Integration Tests

This directory contains comprehensive integration tests for qnap-vm that run against **real QNAP hardware**. These tests validate the complete functionality of the tool in production-like environments.

## Overview

The integration test suite follows the same proven approach as the qnap-docker project, ensuring reliable validation of:

- SSH connection and authentication
- Virtualization Station availability and functionality
- Storage pool detection and management
- Complete VM lifecycle operations (create, start, stop, delete)
- VM configuration and resource management
- Error handling and cleanup procedures

## Prerequisites

### QNAP Device Requirements

- **QNAP NAS** with Virtualization Station installed
- **QTS**: 5.1.0+ or **QuTS hero**: h5.1.0+
- **CPU**: Intel VT-x or AMD-V virtualization support
- **Memory**: Minimum 4GB (8GB+ recommended for testing)
- **SSH access** enabled
- **Virtualization Station** app installed and configured

### Test Environment Setup

1. **Enable SSH** on your QNAP device:
   ```
   Control Panel → Telnet/SSH → Enable SSH service
   ```

2. **Configure SSH key authentication** (recommended):
   ```bash
   ssh-copy-id admin@your-qnap.local
   ```

3. **Install Virtualization Station**:
   ```
   App Center → Virtualization Station → Install
   ```

4. **Verify virtualization support**:
   ```bash
   ssh admin@your-qnap.local "cat /proc/cpuinfo | grep -E '(vmx|svm)'"
   ```

## Running Integration Tests

### Basic Integration Tests

Test against your QNAP device using environment variables:

```bash
# Required: specify your QNAP device
export NAS_HOST=your-qnap.local

# Optional: specify SSH user (defaults to 'admin')
export NAS_USER=admin

# Optional: specify SSH private key path
export NAS_SSH_KEY=~/.ssh/id_rsa

# Run integration tests
make integration-test
```

### Integration Tests with Coverage

Generate detailed coverage reports for integration testing:

```bash
export NAS_HOST=your-qnap.local
make integration-test-full

# View coverage report
open integration_coverage.html
```

### Manual Test Execution

Run tests directly with go test:

```bash
# Run with custom timeout and verbose output
go test -integration -timeout 15m -v ./tests/integration/...

# Run specific test functions
go test -integration -run TestVMLifecycle ./tests/integration/...
```

## Test Structure

### Test Categories

1. **SSH Connection Tests** (`testSSHConnection`)
   - Validates SSH connectivity and authentication
   - Tests basic command execution
   - Verifies system information retrieval

2. **Virtualization Station Tests** (`testVirtualizationStationAvailability`)
   - Confirms Virtualization Station installation
   - Validates virsh/libvirt availability
   - Checks virtualization environment setup

3. **Storage Pool Tests** (`testStoragePoolDetection`)
   - Detects all available storage pools
   - Validates pool properties and metadata
   - Tests storage pool prioritization logic
   - Verifies disk path generation

4. **VM Lifecycle Tests** (`testVMLifecycle`)
   - Creates test VMs with proper configuration
   - Tests VM start/stop operations
   - Validates VM state transitions
   - Ensures complete cleanup

5. **VM Configuration Tests** (`testVMConfiguration`)
   - Validates memory and CPU allocation
   - Tests disk configuration
   - Verifies UUID generation
   - Checks resource constraints

### Test Infrastructure

**TestRunner** provides:
- Automated connection management
- Test VM tracking for cleanup
- Resource management
- Error handling and recovery

**Safety Features**:
- Automatic cleanup of test VMs
- Graceful connection handling
- Resource leak prevention
- Comprehensive error reporting

## Configuration Options

### Environment Variables

| Variable | Description | Default | Required |
|----------|-------------|---------|----------|
| `NAS_HOST` | QNAP device hostname or IP address | - | ✅ |
| `NAS_USER` | SSH username for authentication | `admin` | - |
| `NAS_SSH_KEY` | Path to SSH private key file | - | - |

### Test Timeouts

- **Default timeout**: 10 minutes
- **Connection timeout**: 30 seconds
- **VM operation timeout**: 3 seconds per operation
- **Custom timeout**: Use `-timeout` flag with go test

## Troubleshooting

### Common Issues

**SSH Connection Failed**
```bash
# Verify SSH access
ssh admin@your-qnap.local "echo 'Connection successful'"

# Check SSH key permissions
chmod 600 ~/.ssh/id_rsa
```

**Virtualization Station Not Available**
```bash
# Check if Virtualization Station is installed
ssh admin@your-qnap.local "ls -la /opt/QVS /opt/KVM"

# Verify virtualization support
ssh admin@your-qnap.local "cat /proc/cpuinfo | grep -E '(vmx|svm)'"
```

**Insufficient Storage Space**
- Ensure at least 5GB free space for test VMs
- Check storage pool availability:
```bash
ssh admin@your-qnap.local "df -h"
```

**Test VM Cleanup Issues**
- VMs are automatically cleaned up after tests
- Manual cleanup if needed:
```bash
ssh admin@your-qnap.local "virsh list --all | grep qnap-vm-integration-test"
```

### Test Logs

Integration tests provide detailed logging:

```bash
# Run with verbose logging
go test -integration -v ./tests/integration/...

# Specific test debugging
go test -integration -run TestVMLifecycle -v ./tests/integration/...
```

## Safety Considerations

### Test VM Naming

Test VMs use timestamp-based naming:
- `qnap-vm-integration-test-{timestamp}`
- `qnap-vm-config-test-{timestamp}`

This prevents conflicts with production VMs.

### Resource Usage

Tests create minimal VMs:
- **Memory**: 512MB - 1GB
- **CPUs**: 1-2 cores
- **Disk**: 1-2GB
- **Lifecycle**: Created and destroyed during tests

### Cleanup Guarantees

- All test VMs are tracked and cleaned up automatically
- Cleanup occurs even if tests fail
- Test artifacts are removed from storage pools
- SSH connections are properly closed

## CI/CD Integration

Integration tests are designed to be incorporated into CI/CD pipelines:

```yaml
# Example GitHub Actions integration
- name: Run Integration Tests
  env:
    NAS_HOST: ${{ secrets.QNAP_TEST_HOST }}
    NAS_USER: ${{ secrets.QNAP_USER }}
    NAS_SSH_KEY: ${{ secrets.QNAP_SSH_KEY_PATH }}
  run: make integration-test
```

## Contributing

When adding new integration tests:

1. **Follow the TestRunner pattern** for resource management
2. **Register test VMs** for automatic cleanup
3. **Use descriptive test names** with timestamps
4. **Include comprehensive error messages**
5. **Test both success and failure scenarios**
6. **Document any new environment variables**

## Hardware Compatibility

Tested on QNAP models:
- TS-x64 series (Intel x86_64)
- TS-x77 series (Intel x86_64)
- TVS-x72X series (Intel x86_64)

*ARM-based models are not supported due to Virtualization Station requirements.*

---

**⚠️ Warning**: Integration tests run against real hardware and create actual VMs. Ensure you're running against a test/development QNAP device, not production systems.