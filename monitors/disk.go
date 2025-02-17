// disk.go
package monitors

import (
    "github.com/shirou/gopsutil/v3/disk"
    "sort"
    "strings"
)

type DiskUsageInfo struct {
    Path        string
    UsedPercent float64
    DeviceID    string  // Store the physical device ID
}

func GetTopUsedDisks(count int) []string {
    // Get all partitions
    partitions, err := disk.Partitions(true)  // Changed to true to get all partition info
    if err != nil {
        return []string{"/"}
    }

    var diskUsages []DiskUsageInfo
    seenDevices := make(map[string]bool)  // Track unique physical devices
    
    // Get usage for each partition
    for _, partition := range partitions {
        // Skip special filesystems
        if isSpecialFS(partition.Fstype) {
            continue
        }

        // Extract the base device (e.g., /dev/sda from /dev/sda1)
        baseDevice := getBaseDevice(partition.Device)
        
        // Skip if we've already seen this physical device
        if seenDevices[baseDevice] {
            continue
        }
        
        usage, err := disk.Usage(partition.Mountpoint)
        if err != nil {
            continue
        }
        
        // Only consider if it has actual storage
        if usage.Total > 0 {
            diskUsages = append(diskUsages, DiskUsageInfo{
                Path:        partition.Mountpoint,
                UsedPercent: usage.UsedPercent,
                DeviceID:    baseDevice,
            })
            seenDevices[baseDevice] = true
        }
    }

    // Sort by usage percentage in descending order
    sort.Slice(diskUsages, func(i, j int) bool {
        return diskUsages[i].UsedPercent > diskUsages[j].UsedPercent
    })

    // Get the top N paths
    result := make([]string, 0, count)
    for i := 0; i < count && i < len(diskUsages); i++ {
        result = append(result, diskUsages[i].Path)
    }

    // If we found no valid disks, return root as fallback
    if len(result) == 0 {
        return []string{"/"}
    }

    return result
}

// Helper function to get the base device name
func getBaseDevice(device string) string {
    // Handle cases like /dev/sda1, /dev/nvme0n1p1, etc.
    device = strings.TrimSpace(device)
    
    // Handle NVMe drives
    if strings.Contains(device, "nvme") {
        parts := strings.Split(device, "p")
        if len(parts) > 1 {
            return parts[0]
        }
    }
    
    // Handle traditional drives (sda, hda, etc.)
    for i := len(device) - 1; i >= 0; i-- {
        if device[i] < '0' || device[i] > '9' {
            return device[:i+1]
        }
    }
    
    return device
}

// Helper function to skip special filesystems
func isSpecialFS(fstype string) bool {
    specialFS := map[string]bool{
        "devfs":     true,
        "tmpfs":     true,
        "devtmpfs":  true,
        "proc":      true,
        "sysfs":     true,
        "debugfs":   true,
        "cgroup":    true,
        "securityfs": true,
        "fusectl":   true,
        "pstore":    true,
        "bpf":       true,
        "hugetlbfs": true,
		"squashfs":  true,  // Used by snaps
		"overlay":   true,  // Used by containers and some snap systems
		"fuse":      true,  // FUSE filesystems
		"ecryptfs":  true,  // Encrypted filesystems
		"autofs":    true,  // Automounted filesystems
		"mqueue":    true,  // Message queue filesystem
		"configfs":  true,  // Kernel config filesystem
    }
    return specialFS[fstype] || 
           strings.HasPrefix(fstype, "fuse.") || // Catch all FUSE-based filesystems
           strings.Contains(fstype, "snap")      // Catch any snap-related filesystems
}

func GetDiskUsage(diskPath string) float64 {
    diskStat, err := disk.Usage(diskPath)
    if err != nil {
        return 0.0
    }
    return diskStat.UsedPercent
}

func GetDiskSize(diskPath string) uint64 {
    diskStat, err := disk.Usage(diskPath)
    if err != nil {
        return 0
    }
    return diskStat.Total
}
