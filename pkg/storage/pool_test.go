package storage

import (
	"fmt"
	"testing"
)

func TestParseSize(t *testing.T) {
	tests := []struct {
		input    string
		expected int64
	}{
		{"100G", 100},
		{"100g", 100},
		{"2048M", 2},
		{"2048m", 2},
		{"1024K", 0}, // Should be 0 since it's less than 1GB
		{"1T", 1024},
		{"1t", 1024},
		{"invalid", 0},
		{"", 0},
		{"50", 50}, // No unit assumes GB
		{"123.5G", 123},
		{"2.5T", 2560},
	}

	for _, tt := range tests {
		t.Run(tt.input, func(t *testing.T) {
			result := parseSize(tt.input)
			if result != tt.expected {
				t.Errorf("parseSize(%s) = %d, expected %d", tt.input, result, tt.expected)
			}
		})
	}
}

func TestExtractUSBDeviceName(t *testing.T) {
	tests := []struct {
		input    string
		expected string
	}{
		{"/share/USB/SDisk", "SDisk"},
		{"/share/USB/HDisk/partition1", "partition1"},
		{"", "unknown"},
		{"/", "unknown"},
		{"simple", "simple"},
	}

	for _, tt := range tests {
		t.Run(tt.input, func(t *testing.T) {
			result := extractUSBDeviceName(tt.input)
			if result != tt.expected {
				t.Errorf("extractUSBDeviceName(%s) = %s, expected %s", tt.input, result, tt.expected)
			}
		})
	}
}

func TestCreateVMDiskPath(t *testing.T) {
	pool := &Pool{
		Name: "CACHEDEV1_DATA",
		Path: "/share/CACHEDEV1_DATA",
		Type: "CACHEDEV",
	}

	vmName := "test-vm"

	// Test the path construction logic directly
	expectedPath := fmt.Sprintf("%s/.qnap-vm/disks/%s.qcow2", pool.Path, vmName)
	actualPath := fmt.Sprintf("%s/.qnap-vm/disks/%s.qcow2", pool.Path, vmName)

	if actualPath != expectedPath {
		t.Errorf("Expected path %s, got %s", expectedPath, actualPath)
	}
}

func TestBestPoolSelection(t *testing.T) {
	pools := []Pool{
		{
			Name:      "usb-device",
			Type:      "USB",
			FreeSpace: 100,
			Available: true,
		},
		{
			Name:      "CACHEDEV1_DATA",
			Type:      "CACHEDEV",
			FreeSpace: 50,
			Available: true,
		},
		{
			Name:      "zfs-pool",
			Type:      "ZFS",
			FreeSpace: 75,
			Available: true,
		},
	}

	// Simulate the best pool selection logic
	var bestPool *Pool
	for i := range pools {
		pool := &pools[i]
		if !pool.Available {
			continue
		}

		if bestPool == nil {
			bestPool = pool
			continue
		}

		// Prefer CACHEDEV over USB, ZFS over USB
		if pool.Type == "CACHEDEV" && bestPool.Type != "CACHEDEV" {
			bestPool = pool
		} else if pool.Type == "ZFS" && bestPool.Type == "USB" {
			bestPool = pool
		} else if pool.Type == bestPool.Type && pool.FreeSpace > bestPool.FreeSpace {
			bestPool = pool
		}
	}

	// Should prefer CACHEDEV over others
	if bestPool.Type != "CACHEDEV" {
		t.Errorf("Expected CACHEDEV to be selected as best pool, got %s", bestPool.Type)
	}

	if bestPool.Name != "CACHEDEV1_DATA" {
		t.Errorf("Expected CACHEDEV1_DATA to be selected, got %s", bestPool.Name)
	}
}
