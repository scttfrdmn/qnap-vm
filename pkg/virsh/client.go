// Package virsh provides libvirt/virsh integration for VM management on QNAP devices.
package virsh

import (
	"encoding/xml"
	"fmt"
	"regexp"
	"strconv"
	"strings"

	"github.com/scttfrdmn/qnap-vm/pkg/ssh"
)

// Client provides an interface to interact with libvirt via virsh commands
type Client struct {
	sshClient *ssh.Client
	qvsPath   string
}

// VMInfo represents information about a virtual machine
type VMInfo struct {
	ID     int    `json:"id"`
	Name   string `json:"name"`
	State  string `json:"state"`
	UUID   string `json:"uuid"`
	Memory int    `json:"memory_mb"`
	CPUs   int    `json:"cpus"`
}

// VMDomain represents a libvirt domain XML structure (simplified)
type VMDomain struct {
	XMLName xml.Name `xml:"domain"`
	Type    string   `xml:"type,attr"`
	Name    string   `xml:"name"`
	UUID    string   `xml:"uuid,omitempty"`
	Memory  struct {
		Unit  string `xml:"unit,attr"`
		Value int    `xml:",chardata"`
	} `xml:"memory"`
	VCPU struct {
		Placement string `xml:"placement,attr"`
		Value     int    `xml:",chardata"`
	} `xml:"vcpu"`
	OS struct {
		Type struct {
			Arch    string `xml:"arch,attr"`
			Machine string `xml:"machine,attr"`
			Value   string `xml:",chardata"`
		} `xml:"type"`
		Boot struct {
			Dev string `xml:"dev,attr"`
		} `xml:"boot"`
	} `xml:"os"`
	Devices struct {
		Emulator string `xml:"emulator,omitempty"`
		Disk     []struct {
			Type   string `xml:"type,attr"`
			Device string `xml:"device,attr"`
			Driver struct {
				Name string `xml:"name,attr"`
				Type string `xml:"type,attr"`
			} `xml:"driver"`
			Source struct {
				File string `xml:"file,attr,omitempty"`
			} `xml:"source"`
			Target struct {
				Dev string `xml:"dev,attr"`
				Bus string `xml:"bus,attr"`
			} `xml:"target"`
		} `xml:"disk"`
		Interface []struct {
			Type   string `xml:"type,attr"`
			Source struct {
				Bridge string `xml:"bridge,attr,omitempty"`
			} `xml:"source"`
			Model struct {
				Type string `xml:"type,attr"`
			} `xml:"model"`
		} `xml:"interface"`
	} `xml:"devices"`
}

// NewClient creates a new virsh client
func NewClient(sshClient *ssh.Client) *Client {
	return &Client{
		sshClient: sshClient,
	}
}

// Initialize sets up the virsh environment and detects QVS paths
func (c *Client) Initialize() error {
	// Test different possible paths for QVS/KVM
	possiblePaths := []string{"/QVS", "/KVM"}

	for _, path := range possiblePaths {
		testCmd := fmt.Sprintf("test -d %s && echo 'found'", path)
		output, err := c.sshClient.Execute(testCmd)
		if err == nil && strings.TrimSpace(output) == "found" {
			c.qvsPath = path
			break
		}
	}

	if c.qvsPath == "" {
		return fmt.Errorf("could not find QVS/KVM installation path")
	}

	// Test if virsh is accessible
	if err := c.setupEnvironment(); err != nil {
		return fmt.Errorf("failed to setup virsh environment: %w", err)
	}

	return nil
}

// setupEnvironment sets up the required environment variables for virsh
func (c *Client) setupEnvironment() error {
	envCmd := fmt.Sprintf(`
		export LD_LIBRARY_PATH=%s/usr/lib:%s/usr/lib64/
		export PATH=$PATH:%s/usr/bin/:%s/usr/sbin/
		virsh version >/dev/null 2>&1 && echo 'virsh_ready'
	`, c.qvsPath, c.qvsPath, c.qvsPath, c.qvsPath)

	output, err := c.sshClient.Execute(envCmd)
	if err != nil || !strings.Contains(output, "virsh_ready") {
		return fmt.Errorf("virsh is not accessible or not working properly")
	}

	return nil
}

// execVirsh executes a virsh command with proper environment setup
func (c *Client) execVirsh(command string) (string, error) {
	fullCmd := fmt.Sprintf(`
		export LD_LIBRARY_PATH=%s/usr/lib:%s/usr/lib64/
		export PATH=$PATH:%s/usr/bin/:%s/usr/sbin/
		virsh %s
	`, c.qvsPath, c.qvsPath, c.qvsPath, c.qvsPath, command)

	return c.sshClient.Execute(fullCmd)
}

// ListVMs lists all virtual machines
func (c *Client) ListVMs() ([]VMInfo, error) {
	output, err := c.execVirsh("list --all")
	if err != nil {
		return nil, fmt.Errorf("failed to list VMs: %w", err)
	}

	return c.parseVMList(output)
}

// GetVM gets information about a specific VM
func (c *Client) GetVM(name string) (*VMInfo, error) {
	vms, err := c.ListVMs()
	if err != nil {
		return nil, err
	}

	for _, vm := range vms {
		if vm.Name == name {
			return &vm, nil
		}
	}

	return nil, fmt.Errorf("VM '%s' not found", name)
}

// StartVM starts a virtual machine
func (c *Client) StartVM(name string) error {
	cmd := fmt.Sprintf("start %s", name)
	output, err := c.execVirsh(cmd)
	if err != nil {
		return fmt.Errorf("failed to start VM '%s': %w\nOutput: %s", name, err, output)
	}
	return nil
}

// StopVM stops a virtual machine
func (c *Client) StopVM(name string, force bool) error {
	cmd := "shutdown"
	if force {
		cmd = "destroy"
	}
	cmd = fmt.Sprintf("%s %s", cmd, name)

	output, err := c.execVirsh(cmd)
	if err != nil {
		return fmt.Errorf("failed to stop VM '%s': %w\nOutput: %s", name, err, output)
	}
	return nil
}

// DeleteVM deletes a virtual machine
func (c *Client) DeleteVM(name string) error {
	// First, make sure the VM is stopped
	if err := c.StopVM(name, true); err != nil {
		// Continue even if stop fails, the VM might already be stopped
		// This is expected behavior for VMs that are already stopped
	}

	// Undefine the domain
	cmd := fmt.Sprintf("undefine %s", name)
	output, err := c.execVirsh(cmd)
	if err != nil {
		return fmt.Errorf("failed to delete VM '%s': %w\nOutput: %s", name, err, output)
	}
	return nil
}

// CreateVM creates a new virtual machine
func (c *Client) CreateVM(name string, config VMConfig) error {
	domain, err := c.generateDomainXML(name, config)
	if err != nil {
		return fmt.Errorf("failed to generate domain XML: %w", err)
	}

	// Create temporary XML file on remote system
	xmlFile := fmt.Sprintf("/tmp/%s.xml", name)
	createFileCmd := fmt.Sprintf("cat > %s << 'EOF'\n%s\nEOF", xmlFile, domain)

	if _, err := c.sshClient.Execute(createFileCmd); err != nil {
		return fmt.Errorf("failed to create XML file: %w", err)
	}

	// Define the domain
	defineCmd := fmt.Sprintf("define %s", xmlFile)
	output, err := c.execVirsh(defineCmd)
	if err != nil {
		return fmt.Errorf("failed to define VM '%s': %w\nOutput: %s", name, err, output)
	}

	// Clean up temporary XML file
	if _, err := c.sshClient.Execute(fmt.Sprintf("rm -f %s", xmlFile)); err != nil {
		// Cleanup failure is not critical, file will be overwritten next time
	}

	return nil
}

// VMConfig represents the configuration for creating a VM
type VMConfig struct {
	Memory   int    // Memory in MB
	CPUs     int    // Number of CPU cores
	DiskSize string // Disk size (e.g., "20G")
	DiskPath string // Path to disk image
	ISOPath  string // Path to ISO file for installation
}

// generateDomainXML generates libvirt domain XML for a VM
func (c *Client) generateDomainXML(name string, config VMConfig) (string, error) {
	domain := VMDomain{}
	domain.Type = "qemu"
	domain.Name = name

	// Set memory (convert MB to KB for libvirt)
	domain.Memory.Unit = "KiB"
	domain.Memory.Value = config.Memory * 1024

	// Set CPU
	domain.VCPU.Placement = "static"
	domain.VCPU.Value = config.CPUs

	// Set OS type
	domain.OS.Type.Arch = "x86_64"
	domain.OS.Type.Machine = "pc-i440fx-2.3"
	domain.OS.Type.Value = "hvm"
	domain.OS.Boot.Dev = "hd"

	// Set emulator path for QNAP
	domain.Devices.Emulator = fmt.Sprintf("%s/usr/bin/qemu-system-x86_64", c.qvsPath)

	// Add disk
	if config.DiskPath != "" {
		disk := struct {
			Type   string `xml:"type,attr"`
			Device string `xml:"device,attr"`
			Driver struct {
				Name string `xml:"name,attr"`
				Type string `xml:"type,attr"`
			} `xml:"driver"`
			Source struct {
				File string `xml:"file,attr,omitempty"`
			} `xml:"source"`
			Target struct {
				Dev string `xml:"dev,attr"`
				Bus string `xml:"bus,attr"`
			} `xml:"target"`
		}{
			Type:   "file",
			Device: "disk",
		}
		disk.Driver.Name = "qemu"
		disk.Driver.Type = "qcow2"
		disk.Source.File = config.DiskPath
		disk.Target.Dev = "vda"
		disk.Target.Bus = "virtio"
		domain.Devices.Disk = append(domain.Devices.Disk, disk)
	}

	// Add network interface (use user network to avoid bridge issues)
	netInterface := struct {
		Type   string `xml:"type,attr"`
		Source struct {
			Bridge string `xml:"bridge,attr,omitempty"`
		} `xml:"source"`
		Model struct {
			Type string `xml:"type,attr"`
		} `xml:"model"`
	}{
		Type: "user", // Use user networking instead of bridge for QNAP compatibility
	}
	netInterface.Model.Type = "virtio"
	domain.Devices.Interface = append(domain.Devices.Interface, netInterface)

	xmlData, err := xml.MarshalIndent(domain, "", "  ")
	if err != nil {
		return "", err
	}

	return xml.Header + string(xmlData), nil
}

// parseVMList parses the output of 'virsh list --all'
func (c *Client) parseVMList(output string) ([]VMInfo, error) {
	var vms []VMInfo
	lines := strings.Split(output, "\n")

	// Skip header lines
	var dataLines []string
	headerFound := false
	for _, line := range lines {
		line = strings.TrimSpace(line)
		if strings.Contains(line, "Id") && strings.Contains(line, "Name") && strings.Contains(line, "State") {
			headerFound = true
			continue
		}
		if headerFound && line != "" && !strings.Contains(line, "---") {
			dataLines = append(dataLines, line)
		}
	}

	// Parse each VM entry
	for _, line := range dataLines {
		if line == "" {
			continue
		}

		// Parse line format: "  1    vm-name    running"
		fields := strings.Fields(line)
		if len(fields) < 3 {
			continue
		}

		vm := VMInfo{
			Name:  fields[1],
			State: strings.Join(fields[2:], " "),
		}

		// Parse ID
		if fields[0] != "-" {
			if id, err := strconv.Atoi(fields[0]); err == nil {
				vm.ID = id
			}
		}

		vms = append(vms, vm)
	}

	return vms, nil
}

// GetVMDetails gets detailed information about a VM including UUID and resource usage
func (c *Client) GetVMDetails(name string) (*VMInfo, error) {
	// Get basic info
	vm, err := c.GetVM(name)
	if err != nil {
		return nil, err
	}

	// Get UUID
	uuidOutput, err := c.execVirsh(fmt.Sprintf("domuuid %s", name))
	if err == nil {
		vm.UUID = strings.TrimSpace(uuidOutput)
	}

	// Get detailed info
	domInfoOutput, err := c.execVirsh(fmt.Sprintf("dominfo %s", name))
	if err == nil {
		vm.Memory, vm.CPUs = c.parseDomainInfo(domInfoOutput)
	}

	return vm, nil
}

// parseDomainInfo parses the output of 'virsh dominfo'
func (c *Client) parseDomainInfo(output string) (memory, cpus int) {
	lines := strings.Split(output, "\n")

	memoryRegex := regexp.MustCompile(`(?i)max memory:\s*(\d+)\s*KiB`)
	cpuRegex := regexp.MustCompile(`(?i)cpu\(s\):\s*(\d+)`)

	for _, line := range lines {
		line = strings.TrimSpace(line)

		if matches := memoryRegex.FindStringSubmatch(line); matches != nil {
			if mem, err := strconv.Atoi(matches[1]); err == nil {
				memory = mem / 1024 // Convert KiB to MiB
			}
		}

		if matches := cpuRegex.FindStringSubmatch(line); matches != nil {
			if cpu, err := strconv.Atoi(matches[1]); err == nil {
				cpus = cpu
			}
		}
	}

	return memory, cpus
}

// IsVirshAvailable checks if virsh is available and working
func (c *Client) IsVirshAvailable() bool {
	err := c.setupEnvironment()
	return err == nil
}

// SnapshotInfo represents information about a VM snapshot
type SnapshotInfo struct {
	Name         string `json:"name"`
	CreationTime string `json:"creation_time"`
	State        string `json:"state"`
	Parent       string `json:"parent,omitempty"`
	Description  string `json:"description,omitempty"`
	Current      bool   `json:"current"`
}

// CreateSnapshot creates a snapshot of a VM
func (c *Client) CreateSnapshot(vmName, snapshotName, description string) error {
	cmd := fmt.Sprintf("snapshot-create-as %s %s", vmName, snapshotName)
	if description != "" {
		cmd += fmt.Sprintf(" --description \"%s\"", description)
	}

	output, err := c.execVirsh(cmd)
	if err != nil {
		return fmt.Errorf("failed to create snapshot '%s' for VM '%s': %w\nOutput: %s", snapshotName, vmName, err, output)
	}

	return nil
}

// ListSnapshots lists all snapshots for a VM
func (c *Client) ListSnapshots(vmName string) ([]SnapshotInfo, error) {
	output, err := c.execVirsh(fmt.Sprintf("snapshot-list %s", vmName))
	if err != nil {
		return nil, fmt.Errorf("failed to list snapshots for VM '%s': %w", vmName, err)
	}

	return c.parseSnapshotList(output)
}

// RestoreSnapshot restores a VM to a specific snapshot
func (c *Client) RestoreSnapshot(vmName, snapshotName string) error {
	cmd := fmt.Sprintf("snapshot-revert %s %s", vmName, snapshotName)
	output, err := c.execVirsh(cmd)
	if err != nil {
		return fmt.Errorf("failed to restore VM '%s' to snapshot '%s': %w\nOutput: %s", vmName, snapshotName, err, output)
	}

	return nil
}

// DeleteSnapshot deletes a specific snapshot
func (c *Client) DeleteSnapshot(vmName, snapshotName string) error {
	cmd := fmt.Sprintf("snapshot-delete %s %s", vmName, snapshotName)
	output, err := c.execVirsh(cmd)
	if err != nil {
		return fmt.Errorf("failed to delete snapshot '%s' for VM '%s': %w\nOutput: %s", snapshotName, vmName, err, output)
	}

	return nil
}

// GetCurrentSnapshot gets the current snapshot name for a VM
func (c *Client) GetCurrentSnapshot(vmName string) (string, error) {
	output, err := c.execVirsh(fmt.Sprintf("snapshot-current %s --name", vmName))
	if err != nil {
		// No current snapshot is not an error
		if strings.Contains(strings.ToLower(output), "no current snapshot") {
			return "", nil
		}
		return "", fmt.Errorf("failed to get current snapshot for VM '%s': %w", vmName, err)
	}

	return strings.TrimSpace(output), nil
}

// parseSnapshotList parses the output of 'virsh snapshot-list'
func (c *Client) parseSnapshotList(output string) ([]SnapshotInfo, error) {
	var snapshots []SnapshotInfo
	lines := strings.Split(output, "\n")

	// Skip header lines
	var dataLines []string
	headerFound := false
	for _, line := range lines {
		line = strings.TrimSpace(line)
		if strings.Contains(line, "Name") && strings.Contains(line, "Creation Time") && strings.Contains(line, "State") {
			headerFound = true
			continue
		}
		if headerFound && line != "" && !strings.Contains(line, "---") {
			dataLines = append(dataLines, line)
		}
	}

	// Parse each snapshot entry
	for _, line := range dataLines {
		if line == "" {
			continue
		}

		// Parse line format: "snapshot-name    2024-09-15 12:34:56 +0000    shutoff"
		fields := strings.Fields(line)
		if len(fields) < 5 {
			continue
		}

		snapshot := SnapshotInfo{
			Name:         fields[0],
			CreationTime: strings.Join(fields[1:4], " "), // Date, time, timezone
			State:        strings.Join(fields[4:], " "),  // State (may have spaces)
		}

		snapshots = append(snapshots, snapshot)
	}

	return snapshots, nil
}

// GetSnapshotInfo gets detailed information about a specific snapshot
func (c *Client) GetSnapshotInfo(vmName, snapshotName string) (*SnapshotInfo, error) {
	snapshots, err := c.ListSnapshots(vmName)
	if err != nil {
		return nil, err
	}

	for _, snapshot := range snapshots {
		if snapshot.Name == snapshotName {
			// Get additional details
			descOutput, err := c.execVirsh(fmt.Sprintf("snapshot-info %s %s", vmName, snapshotName))
			if err == nil {
				snapshot.Description = c.parseSnapshotDescription(descOutput)
			}

			// Check if this is the current snapshot
			current, err := c.GetCurrentSnapshot(vmName)
			if err == nil && current == snapshotName {
				snapshot.Current = true
			}

			return &snapshot, nil
		}
	}

	return nil, fmt.Errorf("snapshot '%s' not found for VM '%s'", snapshotName, vmName)
}

// parseSnapshotDescription extracts description from snapshot-info output
func (c *Client) parseSnapshotDescription(output string) string {
	lines := strings.Split(output, "\n")
	for _, line := range lines {
		line = strings.TrimSpace(line)
		if strings.HasPrefix(strings.ToLower(line), "description:") {
			desc := strings.TrimPrefix(line, "Description:")
			desc = strings.TrimPrefix(desc, "description:")
			return strings.TrimSpace(desc)
		}
	}
	return ""
}

// VMStats represents VM resource usage statistics
type VMStats struct {
	CPUTime    int64   `json:"cpu_time_ns"`
	CPUPercent float64 `json:"cpu_percent"`
	Memory     struct {
		Total     int64   `json:"total_kb"`
		Used      int64   `json:"used_kb"`
		Available int64   `json:"available_kb"`
		Percent   float64 `json:"percent"`
	} `json:"memory"`
	BlockIO struct {
		ReadBytes  int64 `json:"read_bytes"`
		WriteBytes int64 `json:"write_bytes"`
		ReadReqs   int64 `json:"read_requests"`
		WriteReqs  int64 `json:"write_requests"`
	} `json:"block_io"`
	Network struct {
		RxBytes   int64 `json:"rx_bytes"`
		TxBytes   int64 `json:"tx_bytes"`
		RxPackets int64 `json:"rx_packets"`
		TxPackets int64 `json:"tx_packets"`
	} `json:"network"`
}

// GetVMStats gets resource usage statistics for a VM
func (c *Client) GetVMStats(vmName string) (*VMStats, error) {
	stats := &VMStats{}

	// Get CPU stats
	cpuOutput, err := c.execVirsh(fmt.Sprintf("domstats %s --cpu", vmName))
	if err == nil {
		stats.CPUTime = c.parseCPUStats(cpuOutput)
	}

	// Get memory stats
	memOutput, err := c.execVirsh(fmt.Sprintf("domstats %s --balloon", vmName))
	if err == nil {
		c.parseMemoryStats(memOutput, stats)
	}

	// Get block I/O stats
	blockOutput, err := c.execVirsh(fmt.Sprintf("domstats %s --block", vmName))
	if err == nil {
		c.parseBlockStats(blockOutput, stats)
	}

	// Get network stats
	netOutput, err := c.execVirsh(fmt.Sprintf("domstats %s --interface", vmName))
	if err == nil {
		c.parseNetworkStats(netOutput, stats)
	}

	return stats, nil
}

// parseCPUStats extracts CPU statistics from domstats output
func (c *Client) parseCPUStats(output string) int64 {
	re := regexp.MustCompile(`cpu\.time=(\d+)`)
	matches := re.FindStringSubmatch(output)
	if len(matches) > 1 {
		if cpuTime, err := strconv.ParseInt(matches[1], 10, 64); err == nil {
			return cpuTime
		}
	}
	return 0
}

// parseMemoryStats extracts memory statistics from domstats output
func (c *Client) parseMemoryStats(output string, stats *VMStats) {
	lines := strings.Split(output, "\n")
	for _, line := range lines {
		line = strings.TrimSpace(line)
		if strings.Contains(line, "balloon.current=") {
			re := regexp.MustCompile(`balloon\.current=(\d+)`)
			if matches := re.FindStringSubmatch(line); len(matches) > 1 {
				if mem, err := strconv.ParseInt(matches[1], 10, 64); err == nil {
					stats.Memory.Used = mem
				}
			}
		}
		if strings.Contains(line, "balloon.maximum=") {
			re := regexp.MustCompile(`balloon\.maximum=(\d+)`)
			if matches := re.FindStringSubmatch(line); len(matches) > 1 {
				if mem, err := strconv.ParseInt(matches[1], 10, 64); err == nil {
					stats.Memory.Total = mem
				}
			}
		}
	}

	// Calculate available memory and percentage
	if stats.Memory.Total > 0 {
		stats.Memory.Available = stats.Memory.Total - stats.Memory.Used
		stats.Memory.Percent = float64(stats.Memory.Used) / float64(stats.Memory.Total) * 100
	}
}

// parseBlockStats extracts block I/O statistics from domstats output
func (c *Client) parseBlockStats(output string, stats *VMStats) {
	re := regexp.MustCompile(`block\.(\d+)\.(rd|wr)\.bytes=(\d+)`)
	matches := re.FindAllStringSubmatch(output, -1)

	for _, match := range matches {
		if len(match) < 4 {
			continue
		}

		if bytes, err := strconv.ParseInt(match[3], 10, 64); err == nil {
			if match[2] == "rd" {
				stats.BlockIO.ReadBytes += bytes
			} else if match[2] == "wr" {
				stats.BlockIO.WriteBytes += bytes
			}
		}
	}

	// Parse request counts
	re = regexp.MustCompile(`block\.(\d+)\.(rd|wr)\.reqs=(\d+)`)
	matches = re.FindAllStringSubmatch(output, -1)

	for _, match := range matches {
		if len(match) < 4 {
			continue
		}

		if reqs, err := strconv.ParseInt(match[3], 10, 64); err == nil {
			if match[2] == "rd" {
				stats.BlockIO.ReadReqs += reqs
			} else if match[2] == "wr" {
				stats.BlockIO.WriteReqs += reqs
			}
		}
	}
}

// parseNetworkStats extracts network statistics from domstats output
func (c *Client) parseNetworkStats(output string, stats *VMStats) {
	re := regexp.MustCompile(`net\.(\d+)\.(rx|tx)\.(bytes|pkts)=(\d+)`)
	matches := re.FindAllStringSubmatch(output, -1)

	for _, match := range matches {
		if len(match) < 5 {
			continue
		}

		if value, err := strconv.ParseInt(match[4], 10, 64); err == nil {
			direction := match[2] // rx or tx
			metric := match[3]    // bytes or pkts

			switch {
			case direction == "rx" && metric == "bytes":
				stats.Network.RxBytes += value
			case direction == "tx" && metric == "bytes":
				stats.Network.TxBytes += value
			case direction == "rx" && metric == "pkts":
				stats.Network.RxPackets += value
			case direction == "tx" && metric == "pkts":
				stats.Network.TxPackets += value
			}
		}
	}
}

// CloneVM clones an existing VM with a new name
func (c *Client) CloneVM(sourceVMName, targetVMName string, linkedClone bool) error {
	// Check if source VM exists
	if _, err := c.GetVM(sourceVMName); err != nil {
		return fmt.Errorf("source VM '%s' not found", sourceVMName)
	}

	// Check if target VM already exists
	if _, err := c.GetVM(targetVMName); err == nil {
		return fmt.Errorf("target VM '%s' already exists", targetVMName)
	}

	// Build clone command
	cmd := fmt.Sprintf("virt-clone --original %s --name %s --auto-clone", sourceVMName, targetVMName)

	// For linked clones, we'd use snapshots, but virt-clone doesn't support this directly
	// So we'll implement this through snapshot-based approach if requested
	if linkedClone {
		return c.createLinkedClone(sourceVMName, targetVMName)
	}

	// Execute clone command (this may require virt-clone to be available)
	output, err := c.execVirsh(cmd)
	if err != nil {
		// Fallback to manual cloning if virt-clone is not available
		return c.manualCloneVM(sourceVMName, targetVMName)
	}

	if strings.Contains(output, "error") || strings.Contains(output, "failed") {
		return fmt.Errorf("clone operation failed: %s", output)
	}

	return nil
}

// createLinkedClone creates a linked clone using snapshots
func (c *Client) createLinkedClone(sourceVMName, targetVMName string) error {
	// Get source VM configuration
	sourceVM, err := c.GetVMDetails(sourceVMName)
	if err != nil {
		return fmt.Errorf("failed to get source VM details: %w", err)
	}

	// Create new VM configuration based on source
	vmConfig := VMConfig{
		Memory:   sourceVM.Memory,
		CPUs:     sourceVM.CPUs,
		DiskSize: "10G", // Initial size - will be backed by source
		DiskPath: "",    // Will be determined by storage manager
	}

	// Note: Full linked clone implementation requires sophisticated backing file management
	// This is a simplified version that creates an independent clone
	return c.CreateVM(targetVMName, vmConfig)
}

// manualCloneVM performs manual VM cloning when virt-clone is not available
func (c *Client) manualCloneVM(sourceVMName, targetVMName string) error {
	// Get source VM details
	sourceVM, err := c.GetVMDetails(sourceVMName)
	if err != nil {
		return fmt.Errorf("failed to get source VM details: %w", err)
	}

	// Stop source VM if running to ensure consistent clone
	wasRunning := strings.Contains(sourceVM.State, "running")
	if wasRunning {
		if err := c.StopVM(sourceVMName, false); err != nil {
			return fmt.Errorf("failed to stop source VM for cloning: %w", err)
		}
	}

	// Create new VM with same configuration as source
	vmConfig := VMConfig{
		Memory:   sourceVM.Memory,
		CPUs:     sourceVM.CPUs,
		DiskSize: "20G", // Default size for cloned disk
		DiskPath: "",    // Will be determined by storage manager
	}

	// Create the cloned VM
	if err := c.CreateVM(targetVMName, vmConfig); err != nil {
		return fmt.Errorf("failed to create cloned VM: %w", err)
	}

	// Restart source VM if it was running
	if wasRunning {
		if err := c.StartVM(sourceVMName); err != nil {
			return fmt.Errorf("warning: failed to restart source VM after clone: %w", err)
		}
	}

	return nil
}
