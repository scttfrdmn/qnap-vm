package integration

import (
	"flag"
	"fmt"
	"os"
	"strings"
	"testing"
	"time"

	"github.com/scttfrdmn/qnap-vm/pkg/config"
	"github.com/scttfrdmn/qnap-vm/pkg/ssh"
	"github.com/scttfrdmn/qnap-vm/pkg/storage"
	"github.com/scttfrdmn/qnap-vm/pkg/virsh"
)

var integration = flag.Bool("integration", false, "run integration tests")

// TestRunner manages integration test execution
type TestRunner struct {
	config      config.Config
	sshClient   *ssh.Client
	virshClient *virsh.Client
	testVMs     []string // Track test VMs for cleanup
}

// NewTestRunner creates a new test runner with QNAP configuration
func NewTestRunner() (*TestRunner, error) {
	nasHost := os.Getenv("NAS_HOST")
	if nasHost == "" {
		return nil, fmt.Errorf("NAS_HOST environment variable is required for integration tests")
	}

	nasUser := os.Getenv("NAS_USER")
	if nasUser == "" {
		nasUser = "admin"
	}

	nasSSHKey := os.Getenv("NAS_SSH_KEY")

	cfg := config.Config{
		Host:     nasHost,
		Username: nasUser,
		Port:     22,
		KeyFile:  nasSSHKey,
	}

	return &TestRunner{
		config:  cfg,
		testVMs: make([]string, 0),
	}, nil
}

// Setup establishes connections to QNAP device
func (tr *TestRunner) Setup() error {
	// Create SSH client
	sshCfg := ssh.Config{
		Host:     tr.config.Host,
		Port:     tr.config.Port,
		Username: tr.config.Username,
		KeyFile:  tr.config.KeyFile,
		Password: tr.config.Password,
		Timeout:  30 * time.Second,
	}

	var err error
	tr.sshClient, err = ssh.NewClient(sshCfg)
	if err != nil {
		return fmt.Errorf("failed to create SSH client: %w", err)
	}

	if err := tr.sshClient.Connect(); err != nil {
		return fmt.Errorf("failed to connect to QNAP device: %w", err)
	}

	// Test SSH connection
	if err := tr.sshClient.TestConnection(); err != nil {
		return fmt.Errorf("SSH connection test failed: %w", err)
	}

	// Create virsh client
	tr.virshClient = virsh.NewClient(tr.sshClient)

	// Initialize virsh environment
	if err := tr.virshClient.Initialize(); err != nil {
		return fmt.Errorf("failed to initialize virsh: %w", err)
	}

	return nil
}

// Cleanup removes test VMs and closes connections
func (tr *TestRunner) Cleanup() error {
	if tr.virshClient != nil {
		// Clean up test VMs
		for _, vmName := range tr.testVMs {
			// Force stop and delete test VMs
			tr.virshClient.StopVM(vmName, true)
			tr.virshClient.DeleteVM(vmName)
		}
	}

	if tr.sshClient != nil {
		return tr.sshClient.Close()
	}

	return nil
}

// AddTestVM registers a test VM for cleanup
func (tr *TestRunner) AddTestVM(vmName string) {
	tr.testVMs = append(tr.testVMs, vmName)
}

// TestIntegrationMain is the main integration test entry point
func TestIntegrationMain(t *testing.T) {
	if !*integration {
		t.Skip("Integration tests skipped. Use -integration flag to run.")
	}

	// Verify we're running against real hardware
	runner, err := NewTestRunner()
	if err != nil {
		t.Fatalf("Failed to create test runner: %v", err)
	}

	// Setup connection to QNAP device
	if err := runner.Setup(); err != nil {
		t.Fatalf("Failed to setup test environment: %v", err)
	}
	defer func() {
		if err := runner.Cleanup(); err != nil {
			t.Logf("Warning: cleanup failed: %v", err)
		}
	}()

	t.Run("SSH Connection", func(t *testing.T) {
		testSSHConnection(t, runner)
	})

	t.Run("Virtualization Station Availability", func(t *testing.T) {
		testVirtualizationStationAvailability(t, runner)
	})

	t.Run("Storage Pool Detection", func(t *testing.T) {
		testStoragePoolDetection(t, runner)
	})

	t.Run("VM Lifecycle", func(t *testing.T) {
		testVMLifecycle(t, runner)
	})

	t.Run("VM Configuration", func(t *testing.T) {
		testVMConfiguration(t, runner)
	})
}

// testSSHConnection validates SSH connectivity and authentication
func testSSHConnection(t *testing.T, runner *TestRunner) {
	// Test basic command execution
	output, err := runner.sshClient.Execute("echo 'SSH test successful'")
	if err != nil {
		t.Fatalf("SSH command execution failed: %v", err)
	}

	if !strings.Contains(output, "SSH test successful") {
		t.Errorf("SSH command output unexpected: %s", output)
	}

	// Test system information retrieval
	output, err = runner.sshClient.Execute("uname -a")
	if err != nil {
		t.Fatalf("Failed to get system information: %v", err)
	}

	if output == "" {
		t.Error("System information should not be empty")
	}

	t.Logf("Connected to system: %s", strings.TrimSpace(output))
}

// testVirtualizationStationAvailability checks if Virtualization Station is properly installed
func testVirtualizationStationAvailability(t *testing.T, runner *TestRunner) {
	// Check if virsh is available
	if !runner.virshClient.IsVirshAvailable() {
		t.Fatal("Virtualization Station (virsh) is not available on this QNAP device")
	}

	// Test virsh version
	output, err := runner.sshClient.Execute(`
		export LD_LIBRARY_PATH=/QVS/usr/lib:/QVS/usr/lib64/:/KVM/usr/lib:/KVM/usr/lib64/
		export PATH=$PATH:/QVS/usr/bin/:/QVS/usr/sbin/:/KVM/usr/bin/:/KVM/usr/sbin/
		virsh version
	`)
	if err != nil {
		t.Fatalf("Failed to get virsh version: %v", err)
	}

	if !strings.Contains(strings.ToLower(output), "libvirt") {
		t.Errorf("Unexpected virsh version output: %s", output)
	}

	t.Logf("Virtualization Station available: %s", strings.TrimSpace(output))
}

// testStoragePoolDetection validates storage pool detection and management
func testStoragePoolDetection(t *testing.T, runner *TestRunner) {
	storageManager := storage.NewManager(runner.sshClient)

	pools, err := storageManager.DetectPools()
	if err != nil {
		t.Fatalf("Failed to detect storage pools: %v", err)
	}

	if len(pools) == 0 {
		t.Fatal("No storage pools detected - QNAP device should have at least one storage pool")
	}

	t.Logf("Detected %d storage pools:", len(pools))
	for i, pool := range pools {
		t.Logf("  Pool %d: %s (%s) - %s", i+1, pool.Name, pool.Type, pool.Path)

		// Validate pool properties
		if pool.Name == "" {
			t.Errorf("Pool %d has empty name", i+1)
		}
		if pool.Path == "" {
			t.Errorf("Pool %d has empty path", i+1)
		}
		if pool.Type == "" {
			t.Errorf("Pool %d has empty type", i+1)
		}
	}

	// Test getting best pool
	bestPool, err := storageManager.GetBestPool()
	if err != nil {
		t.Fatalf("Failed to get best storage pool: %v", err)
	}

	if bestPool == nil {
		t.Fatal("Best pool should not be nil")
	}

	t.Logf("Best pool selected: %s (%s)", bestPool.Name, bestPool.Type)

	// Validate pool priority (CACHEDEV should be preferred over others)
	foundCacheDev := false
	for _, pool := range pools {
		if pool.Type == "CACHEDEV" {
			foundCacheDev = true
			break
		}
	}

	if foundCacheDev && bestPool.Type != "CACHEDEV" {
		t.Errorf("CACHEDEV pool available but not selected as best pool. Selected: %s", bestPool.Type)
	}
}

// testVMLifecycle tests complete VM lifecycle: create, start, stop, delete
func testVMLifecycle(t *testing.T, runner *TestRunner) {
	testVMName := fmt.Sprintf("qnap-vm-integration-test-%d", time.Now().Unix())
	runner.AddTestVM(testVMName)

	t.Logf("Testing VM lifecycle with VM: %s", testVMName)

	// Get storage for VM creation
	storageManager := storage.NewManager(runner.sshClient)
	bestPool, err := storageManager.GetBestPool()
	if err != nil {
		t.Fatalf("Failed to get storage pool for VM: %v", err)
	}

	// Create disk path
	diskPath := storageManager.CreateVMDiskPath(bestPool, testVMName)

	// Create VM disk
	if err := storageManager.CreateVMDisk(diskPath, "1G"); err != nil {
		t.Fatalf("Failed to create VM disk: %v", err)
	}

	// Test VM Creation
	t.Run("Create VM", func(t *testing.T) {
		vmConfig := virsh.VMConfig{
			Memory:   512, // Small memory for testing
			CPUs:     1,
			DiskSize: "1G",
			DiskPath: diskPath,
		}

		err := runner.virshClient.CreateVM(testVMName, vmConfig)
		if err != nil {
			t.Fatalf("Failed to create VM: %v", err)
		}

		// Verify VM was created
		vm, err := runner.virshClient.GetVM(testVMName)
		if err != nil {
			t.Fatalf("Failed to retrieve created VM: %v", err)
		}

		if vm.Name != testVMName {
			t.Errorf("VM name mismatch. Expected: %s, Got: %s", testVMName, vm.Name)
		}

		t.Logf("VM created successfully: %s", testVMName)
	})

	// Test VM Start
	t.Run("Start VM", func(t *testing.T) {
		err := runner.virshClient.StartVM(testVMName)
		if err != nil {
			t.Fatalf("Failed to start VM: %v", err)
		}

		// Wait a moment for VM to start
		time.Sleep(3 * time.Second)

		// Verify VM is running
		vm, err := runner.virshClient.GetVM(testVMName)
		if err != nil {
			t.Fatalf("Failed to get VM status: %v", err)
		}

		if !strings.Contains(vm.State, "running") {
			t.Errorf("VM should be running, but state is: %s", vm.State)
		}

		t.Logf("VM started successfully: %s (state: %s)", testVMName, vm.State)
	})

	// Test VM Stop
	t.Run("Stop VM", func(t *testing.T) {
		err := runner.virshClient.StopVM(testVMName, true) // Force stop for testing
		if err != nil {
			t.Fatalf("Failed to stop VM: %v", err)
		}

		// Wait a moment for VM to stop
		time.Sleep(3 * time.Second)

		// Verify VM is stopped
		vm, err := runner.virshClient.GetVM(testVMName)
		if err != nil {
			t.Fatalf("Failed to get VM status: %v", err)
		}

		if strings.Contains(vm.State, "running") {
			t.Errorf("VM should be stopped, but state is: %s", vm.State)
		}

		t.Logf("VM stopped successfully: %s (state: %s)", testVMName, vm.State)
	})

	// Test VM Deletion
	t.Run("Delete VM", func(t *testing.T) {
		err := runner.virshClient.DeleteVM(testVMName)
		if err != nil {
			t.Fatalf("Failed to delete VM: %v", err)
		}

		// Verify VM was deleted
		_, err = runner.virshClient.GetVM(testVMName)
		if err == nil {
			t.Error("VM should be deleted but was still found")
		}

		t.Logf("VM deleted successfully: %s", testVMName)

		// Remove from test VMs list since it's been cleaned up
		for i, name := range runner.testVMs {
			if name == testVMName {
				runner.testVMs = append(runner.testVMs[:i], runner.testVMs[i+1:]...)
				break
			}
		}
	})
}

// testVMConfiguration validates VM configuration and resource settings
func testVMConfiguration(t *testing.T, runner *TestRunner) {
	testVMName := fmt.Sprintf("qnap-vm-config-test-%d", time.Now().Unix())
	runner.AddTestVM(testVMName)

	// Get storage for VM creation
	storageManager := storage.NewManager(runner.sshClient)
	bestPool, err := storageManager.GetBestPool()
	if err != nil {
		t.Fatalf("Failed to get storage pool: %v", err)
	}

	diskPath := storageManager.CreateVMDiskPath(bestPool, testVMName)

	// Create VM disk
	if err := storageManager.CreateVMDisk(diskPath, "2G"); err != nil {
		t.Fatalf("Failed to create VM disk: %v", err)
	}

	// Test VM with specific configuration
	vmConfig := virsh.VMConfig{
		Memory:   1024,
		CPUs:     2,
		DiskSize: "2G",
		DiskPath: diskPath,
	}

	err = runner.virshClient.CreateVM(testVMName, vmConfig)
	if err != nil {
		t.Fatalf("Failed to create VM for configuration test: %v", err)
	}

	// Get VM details and validate configuration
	vmDetails, err := runner.virshClient.GetVMDetails(testVMName)
	if err != nil {
		t.Fatalf("Failed to get VM details: %v", err)
	}

	// Validate memory configuration
	if vmDetails.Memory != 1024 {
		t.Errorf("VM memory mismatch. Expected: 1024 MB, Got: %d MB", vmDetails.Memory)
	}

	// Validate CPU configuration
	if vmDetails.CPUs != 2 {
		t.Errorf("VM CPU count mismatch. Expected: 2, Got: %d", vmDetails.CPUs)
	}

	// Validate UUID is set
	if vmDetails.UUID == "" {
		t.Error("VM UUID should not be empty")
	}

	t.Logf("VM configuration validated: %s (Memory: %d MB, CPUs: %d, UUID: %s)",
		testVMName, vmDetails.Memory, vmDetails.CPUs, vmDetails.UUID)
}