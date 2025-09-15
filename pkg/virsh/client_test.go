package virsh

import (
	"strings"
	"testing"
)

func TestParseVMList(t *testing.T) {
	// Mock output from 'virsh list --all'
	sampleOutput := ` Id   Name       State
----------------------------
 1    vm1        running
 2    vm2        running
-    vm3        shut off
-    vm4        shut off`

	client := &Client{}
	vms, err := client.parseVMList(sampleOutput)
	if err != nil {
		t.Fatalf("parseVMList failed: %v", err)
	}

	expectedCount := 4
	if len(vms) != expectedCount {
		t.Errorf("Expected %d VMs, got %d", expectedCount, len(vms))
	}

	// Test first VM (running)
	if len(vms) > 0 {
		if vms[0].Name != "vm1" {
			t.Errorf("Expected VM name 'vm1', got '%s'", vms[0].Name)
		}
		if vms[0].ID != 1 {
			t.Errorf("Expected VM ID 1, got %d", vms[0].ID)
		}
		if vms[0].State != "running" {
			t.Errorf("Expected VM state 'running', got '%s'", vms[0].State)
		}
	}

	// Test third VM (shut off, no ID) if it exists
	if len(vms) > 2 {
		if vms[2].Name != "vm3" {
			t.Errorf("Expected VM name 'vm3', got '%s'", vms[2].Name)
		}
		if vms[2].ID != 0 {
			t.Errorf("Expected VM ID 0 (unset), got %d", vms[2].ID)
		}
		if vms[2].State != "shut off" {
			t.Errorf("Expected VM state 'shut off', got '%s'", vms[2].State)
		}
	}
}

func TestParseDomainInfo(t *testing.T) {
	// Mock output from 'virsh dominfo'
	sampleOutput := `
Id:             1
Name:           test-vm
UUID:           12345678-1234-1234-1234-123456789abc
OS Type:        hvm
State:          running
CPU(s):         4
CPU time:       123.4s
Max memory:     4194304 KiB
Used memory:    2097152 KiB
Persistent:     yes
Autostart:      disable
Managed save:   no
Security model: none
Security DOI:   0
`

	client := &Client{}
	memory, cpus := client.parseDomainInfo(sampleOutput)

	expectedMemory := 4096 // 4194304 KiB / 1024 = 4096 MiB
	expectedCPUs := 4

	if memory != expectedMemory {
		t.Errorf("Expected memory %d MiB, got %d MiB", expectedMemory, memory)
	}

	if cpus != expectedCPUs {
		t.Errorf("Expected %d CPUs, got %d CPUs", expectedCPUs, cpus)
	}
}

func TestGenerateDomainXML(t *testing.T) {
	client := &Client{}

	config := VMConfig{
		Memory:   2048,
		CPUs:     2,
		DiskSize: "20G",
		DiskPath: "/share/CACHEDEV1_DATA/.qnap-vm/disks/test-vm.qcow2",
	}

	xml, err := client.generateDomainXML("test-vm", config)
	if err != nil {
		t.Fatalf("generateDomainXML failed: %v", err)
	}

	// Check that XML contains expected elements (allowing for different formatting)
	expectedElements := []string{
		"<name>test-vm</name>",
		"<memory unit=\"KiB\">2097152</memory>", // 2048 * 1024
		"<vcpu placement=\"static\">2</vcpu>",
		"<type arch=\"x86_64\" machine=\"pc-i440fx-2.3\">hvm</type>",
		"<source file=\"/share/CACHEDEV1_DATA/.qnap-vm/disks/test-vm.qcow2\">",
		"<target dev=\"vda\" bus=\"virtio\">",
	}

	for _, expected := range expectedElements {
		if !strings.Contains(xml, expected) {
			t.Errorf("Generated XML missing expected element: %s\nGenerated XML:\n%s", expected, xml)
		}
	}

	// Ensure XML starts with declaration
	if !strings.HasPrefix(xml, "<?xml version=\"1.0\" encoding=\"UTF-8\"?>") {
		t.Error("Generated XML should start with XML declaration")
	}
}

func TestVMConfig(t *testing.T) {
	config := VMConfig{
		Memory:   4096,
		CPUs:     4,
		DiskSize: "50G",
		DiskPath: "/path/to/disk.qcow2",
		ISOPath:  "/path/to/installer.iso",
	}

	// Test that all fields are set correctly
	if config.Memory != 4096 {
		t.Errorf("Expected memory 4096, got %d", config.Memory)
	}

	if config.CPUs != 4 {
		t.Errorf("Expected 4 CPUs, got %d", config.CPUs)
	}

	if config.DiskSize != "50G" {
		t.Errorf("Expected disk size 50G, got %s", config.DiskSize)
	}

	if config.DiskPath != "/path/to/disk.qcow2" {
		t.Errorf("Expected disk path /path/to/disk.qcow2, got %s", config.DiskPath)
	}

	if config.ISOPath != "/path/to/installer.iso" {
		t.Errorf("Expected ISO path /path/to/installer.iso, got %s", config.ISOPath)
	}
}
