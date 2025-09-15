// Package storage provides storage pool detection and management for QNAP devices.
package storage

import (
	"fmt"
	"regexp"
	"strconv"
	"strings"

	"github.com/scttfrdmn/qnap-vm/pkg/ssh"
)

// Pool represents a storage pool on QNAP
type Pool struct {
	Name        string `json:"name"`
	Path        string `json:"path"`
	Type        string `json:"type"`
	TotalSpace  int64  `json:"total_space_gb"`
	UsedSpace   int64  `json:"used_space_gb"`
	FreeSpace   int64  `json:"free_space_gb"`
	Available   bool   `json:"available"`
	Description string `json:"description"`
}

// Manager handles storage pool detection and management
type Manager struct {
	sshClient *ssh.Client
}

// NewManager creates a new storage manager
func NewManager(sshClient *ssh.Client) *Manager {
	return &Manager{
		sshClient: sshClient,
	}
}

// DetectPools detects available storage pools on the QNAP device
func (m *Manager) DetectPools() ([]Pool, error) {
	pools := []Pool{}

	// Detect different types of storage pools
	cachePools, err := m.detectCacheDevPools()
	if err == nil {
		pools = append(pools, cachePools...)
	}

	zfsPools, err := m.detectZFSPools()
	if err == nil {
		pools = append(pools, zfsPools...)
	}

	usbPools, err := m.detectUSBPools()
	if err == nil {
		pools = append(pools, usbPools...)
	}

	// Get disk usage for each pool
	for i := range pools {
		if usage, err := m.getDiskUsage(pools[i].Path); err == nil {
			pools[i].TotalSpace = usage.Total
			pools[i].UsedSpace = usage.Used
			pools[i].FreeSpace = usage.Free
		}
	}

	return pools, nil
}

// DiskUsage represents disk usage information
type DiskUsage struct {
	Total int64 // Total space in GB
	Used  int64 // Used space in GB
	Free  int64 // Free space in GB
}

// detectCacheDevPools detects CACHEDEV storage pools
func (m *Manager) detectCacheDevPools() ([]Pool, error) {
	var pools []Pool

	// Look for CACHEDEV directories
	output, err := m.sshClient.Execute("ls -la /share/ | grep CACHEDEV")
	if err != nil {
		return pools, nil // Not an error if no CACHEDEV found
	}

	lines := strings.Split(output, "\n")
	for _, line := range lines {
		if strings.Contains(line, "CACHEDEV") {
			fields := strings.Fields(line)
			if len(fields) >= 9 {
				deviceName := fields[8]
				if strings.HasPrefix(deviceName, "CACHEDEV") {
					pool := Pool{
						Name:        deviceName,
						Path:        fmt.Sprintf("/share/%s_DATA", deviceName),
						Type:        "CACHEDEV",
						Available:   true,
						Description: fmt.Sprintf("QNAP Cache Device Storage - %s", deviceName),
					}
					pools = append(pools, pool)
				}
			}
		}
	}

	return pools, nil
}

// detectZFSPools detects ZFS storage pools
func (m *Manager) detectZFSPools() ([]Pool, error) {
	var pools []Pool

	// Check if ZFS is available
	_, err := m.sshClient.Execute("which zpool")
	if err != nil {
		return pools, nil // ZFS not available
	}

	// List ZFS pools
	output, err := m.sshClient.Execute("zpool list -H")
	if err != nil {
		return pools, nil
	}

	lines := strings.Split(output, "\n")
	for _, line := range lines {
		line = strings.TrimSpace(line)
		if line == "" {
			continue
		}

		fields := strings.Fields(line)
		if len(fields) >= 4 {
			poolName := fields[0]
			pool := Pool{
				Name:        fmt.Sprintf("zfs-%s", poolName),
				Path:        fmt.Sprintf("/share/%s", poolName),
				Type:        "ZFS",
				Available:   true,
				Description: fmt.Sprintf("ZFS Storage Pool - %s", poolName),
			}

			// Parse size if available
			if size := parseSize(fields[1]); size > 0 {
				pool.TotalSpace = size
			}

			pools = append(pools, pool)
		}
	}

	return pools, nil
}

// detectUSBPools detects USB storage devices
func (m *Manager) detectUSBPools() ([]Pool, error) {
	var pools []Pool

	// Look for USB mount points
	output, err := m.sshClient.Execute("mount | grep usb")
	if err != nil {
		return pools, nil
	}

	lines := strings.Split(output, "\n")
	for _, line := range lines {
		line = strings.TrimSpace(line)
		if line == "" {
			continue
		}

		// Parse mount output: /dev/sda1 on /share/USB/SDisk type ext4 (rw,relatime)
		parts := strings.Split(line, " on ")
		if len(parts) >= 2 {
			mountPoint := strings.Split(parts[1], " ")[0]
			if strings.Contains(mountPoint, "USB") {
				deviceName := extractUSBDeviceName(mountPoint)
				pool := Pool{
					Name:        fmt.Sprintf("usb-%s", deviceName),
					Path:        mountPoint,
					Type:        "USB",
					Available:   true,
					Description: fmt.Sprintf("USB Storage Device - %s", deviceName),
				}
				pools = append(pools, pool)
			}
		}
	}

	return pools, nil
}

// getDiskUsage gets disk usage information for a path
func (m *Manager) getDiskUsage(path string) (DiskUsage, error) {
	var usage DiskUsage

	// Use df command to get disk usage
	cmd := fmt.Sprintf("df -BG %s | tail -n 1", path)
	output, err := m.sshClient.Execute(cmd)
	if err != nil {
		return usage, err
	}

	// Parse df output: /dev/md0    123G   45G   67G  41% /share/CACHEDEV1_DATA
	fields := strings.Fields(strings.TrimSpace(output))
	if len(fields) >= 4 {
		if total := parseSize(fields[1]); total > 0 {
			usage.Total = total
		}
		if used := parseSize(fields[2]); used > 0 {
			usage.Used = used
		}
		if free := parseSize(fields[3]); free > 0 {
			usage.Free = free
		}
	}

	return usage, nil
}

// GetBestPool returns the best available pool for VM storage
func (m *Manager) GetBestPool() (*Pool, error) {
	pools, err := m.DetectPools()
	if err != nil {
		return nil, err
	}

	if len(pools) == 0 {
		return nil, fmt.Errorf("no storage pools found")
	}

	// Prioritize pools by type and free space
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
			// Same type, prefer more free space
			bestPool = pool
		}
	}

	if bestPool == nil {
		return nil, fmt.Errorf("no available storage pools found")
	}

	return bestPool, nil
}

// CreateVMDiskPath creates a disk path for a VM in the specified pool
func (m *Manager) CreateVMDiskPath(pool *Pool, vmName string) string {
	// Create a subdirectory for VM disks if it doesn't exist
	vmDir := fmt.Sprintf("%s/.qnap-vm/disks", pool.Path)
	diskPath := fmt.Sprintf("%s/%s.qcow2", vmDir, vmName)

	// Create the directory (ignore errors if it already exists)
	if _, err := m.sshClient.Execute(fmt.Sprintf("mkdir -p %s", vmDir)); err != nil {
		// Directory creation failure is not critical for path generation
		// The actual mkdir will be attempted during VM creation
	}

	return diskPath
}

// CreateVMDisk creates a disk image for a VM
func (m *Manager) CreateVMDisk(diskPath, size string) error {
	// Use qemu-img to create the disk image
	// We'll need to determine if qemu-img is available in the QVS/KVM path
	possiblePaths := []string{"/QVS/usr/bin", "/KVM/usr/bin"}

	var qemuImgPath string
	for _, path := range possiblePaths {
		testCmd := fmt.Sprintf("test -x %s/qemu-img && echo 'found'", path)
		if output, err := m.sshClient.Execute(testCmd); err == nil && strings.Contains(output, "found") {
			qemuImgPath = fmt.Sprintf("%s/qemu-img", path)
			break
		}
	}

	if qemuImgPath == "" {
		return fmt.Errorf("qemu-img not found in expected paths")
	}

	// Create the disk image
	cmd := fmt.Sprintf("%s create -f qcow2 %s %s", qemuImgPath, diskPath, size)
	output, err := m.sshClient.Execute(cmd)
	if err != nil {
		return fmt.Errorf("failed to create disk image: %w\nOutput: %s", err, output)
	}

	return nil
}

// parseSize parses a size string like "123G", "456M", "789K" and returns size in GB
func parseSize(sizeStr string) int64 {
	if sizeStr == "" {
		return 0
	}

	// Remove trailing 'G', 'M', 'K', 'B'
	re := regexp.MustCompile(`(\d+(?:\.\d+)?)\s*([KMGT]?)B?`)
	matches := re.FindStringSubmatch(strings.ToUpper(sizeStr))

	if len(matches) < 2 {
		return 0
	}

	value, err := strconv.ParseFloat(matches[1], 64)
	if err != nil {
		return 0
	}

	unit := ""
	if len(matches) > 2 {
		unit = matches[2]
	}

	// Convert to GB
	switch unit {
	case "K":
		return int64(value / (1024 * 1024))
	case "M":
		return int64(value / 1024)
	case "G", "":
		return int64(value)
	case "T":
		return int64(value * 1024)
	default:
		return int64(value)
	}
}

// extractUSBDeviceName extracts device name from USB mount point
func extractUSBDeviceName(mountPoint string) string {
	parts := strings.Split(mountPoint, "/")
	if len(parts) > 0 && parts[len(parts)-1] != "" {
		return parts[len(parts)-1]
	}
	return "unknown"
}
