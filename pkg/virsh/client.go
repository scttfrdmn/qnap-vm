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
		Disk []struct {
			Type   string `xml:"type,attr"`
			Device string `xml:"device,attr"`
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

	// Add disk
	if config.DiskPath != "" {
		disk := struct {
			Type   string `xml:"type,attr"`
			Device string `xml:"device,attr"`
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
		disk.Source.File = config.DiskPath
		disk.Target.Dev = "vda"
		disk.Target.Bus = "virtio"
		domain.Devices.Disk = append(domain.Devices.Disk, disk)
	}

	// Add network interface
	netInterface := struct {
		Type   string `xml:"type,attr"`
		Source struct {
			Bridge string `xml:"bridge,attr,omitempty"`
		} `xml:"source"`
		Model struct {
			Type string `xml:"type,attr"`
		} `xml:"model"`
	}{
		Type: "bridge",
	}
	netInterface.Source.Bridge = "virbr0" // Default bridge
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
