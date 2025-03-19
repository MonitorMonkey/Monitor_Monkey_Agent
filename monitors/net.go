package monitors

import (
    "fmt"
    "github.com/shirou/gopsutil/v3/net"
    "strings"
)

func GetNetStats() (uint64, uint64) {
    // Get stats for all interfaces (true = per interface)
    nstats, err := net.IOCounters(true)
    if err != nil {
        fmt.Println(err)
        return 0, 0
    }

    var total_upload uint64 = 0
    var total_download uint64 = 0

    // Iterate through all interfaces and sum up the stats
    // Skip the loopback interface (typically named "lo")
    for _, stat := range nstats {
        // Skip loopback interface (usually named "lo" on Linux, "lo0" on macOS)
        if strings.Contains(strings.ToLower(stat.Name), "lo") {
            continue
        }
        
        total_upload += stat.BytesSent
        total_download += stat.BytesRecv
    }

    return total_upload, total_download
}
