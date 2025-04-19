package main

import (
	"encoding/json"
	"flag"
	"fmt"
	"log"
	"os/user"
	"sort"
	"time"

	"github.com/shirou/gopsutil/v3/cpu"
	"github.com/shirou/gopsutil/v3/process"
)

// ProcessMetrics stores the collected stats for a process during the run
type ProcessMetrics struct {
	PID             int32
	Name            string
	Username        string        // User owning the process
	InitialCPUTimes *cpu.TimesStat // Process CPU times at start/first seen
	LastCPUTimes    *cpu.TimesStat // Process CPU times at end/last seen
	PeakMemoryRSS   uint64         // Peak RSS in bytes
	SampleCount     int            // How many times we sampled this process
	LastSeen        time.Time      // Timestamp of the last sample
}

// ProcessCPUStat holds formatted CPU results for JSON output
type ProcessCPUStat struct {
	PID         int32   `json:"pid"`
	Name        string  `json:"name"`
	Username    string  `json:"username"`
	AvgCPUPercent float64 `json:"avg_cpu_percent"`
}

// ProcessMemStat holds formatted Memory results for JSON output
type ProcessMemStat struct {
	PID         int32  `json:"pid"`
	Name        string `json:"name"`
	Username    string `json:"username"`
	PeakRSS_KB  uint64 `json:"peak_rss_kb"`
}

// Result holds the final sorted lists for JSON output
type Result struct {
	StartTime       time.Time        `json:"start_time"`
	EndTime         time.Time        `json:"end_time"`
	Duration        string           `json:"duration"`
	TopCPUProcesses []ProcessCPUStat `json:"top_cpu_processes"`
	TopMemProcesses []ProcessMemStat `json:"top_mem_processes"`
}

// Helper function to calculate total CPU time from cpu.TimesStat
// Returns time in seconds.
func getTotalCPUTime(t *cpu.TimesStat) float64 {
	return t.User + t.System + t.Idle + t.Nice + t.Iowait + t.Irq +
		t.Softirq + t.Steal + t.Guest + t.GuestNice
}

// Helper function to calculate process CPU time (User + System)
// Returns time in seconds.
func getProcessCPUTime(t *cpu.TimesStat) float64 {
	if t == nil {
		return 0
	}
	// User and System are the primary ones we care about for process load
	return t.User + t.System
}


func main() {
	durationFlag := flag.Duration("duration", 60*time.Second, "Monitoring duration (e.g., 30s, 5m, 1h)")
	topNFlag := flag.Int("top", 10, "Number of top processes to report for CPU and Memory")
	sampleIntervalFlag := flag.Duration("interval", 5*time.Second, "Sampling interval (e.g., 1s, 5s)")
	flag.Parse()

	monitorDuration := *durationFlag
	sampleInterval := *sampleIntervalFlag
	topN := *topNFlag

	if monitorDuration <= 0 || sampleInterval <= 0 || topN <= 0 {
		log.Fatalf("Invalid duration, interval, or topN value.")
	}
	if sampleInterval > monitorDuration {
		log.Fatalf("Sample interval cannot be greater than monitoring duration.")
	}

	// Get current user's UID
	currentUser, err := user.Current()
	if err != nil {
		log.Fatalf("Could not get current user: %v", err)
	}
	currentUIDStr := currentUser.Uid
	// Convert currentUIDStr to int32 if needed for direct comparison, though gopsutil might return string UIDs too.
	// For simplicity, we'll often get the username which gopsutil provides.

	log.Printf("Starting CPU/Memory monitor for %v (sampling every %v)", monitorDuration, sampleInterval)
	log.Printf("Monitoring processes for user: %s (UID: %s)", currentUser.Username, currentUIDStr)
	log.Printf("Reporting top %d processes.", topN)

	startTime := time.Now()
	endTime := startTime.Add(monitorDuration)

	// Get initial *total* system CPU times
	initialSystemCPU, err := cpu.Times(false) // false = aggregate over all cores
	if err != nil || len(initialSystemCPU) == 0 {
		log.Fatalf("Failed to get initial system CPU stats: %v", err)
	}
	initialSystemTotalTime := getTotalCPUTime(&initialSystemCPU[0])

	// Map to store process metrics, key is PID
	processData := make(map[int32]*ProcessMetrics)

	ticker := time.NewTicker(sampleInterval)
	defer ticker.Stop()

	// --- Monitoring Loop ---
monitorLoop:
	for now := range ticker.C {
		if now.After(endTime) {
			break monitorLoop
		}

		pids, err := process.Pids()
		if err != nil {
			log.Printf("Warning: Failed to list PIDs: %v", err)
			continue
		}

		currentPIDs := make(map[int32]bool) // Keep track of PIDs seen in this sample

		for _, pid := range pids {
			currentPIDs[pid] = true
			p, err := process.NewProcess(pid)
			if err != nil {
				// Process likely exited or permission denied (shouldn't happen for owned)
				continue
			}

			// Check ownership - use Username for simplicity
			username, err := p.Username()
			if err != nil || username != currentUser.Username {
				continue // Skip processes not owned by the current user
			}

			// Get required stats
			name, _ := p.Name() // Ignore error for name
			cpuTimes, errCPU := p.Times()
			memInfo, errMem := p.MemoryInfo()

			if errCPU != nil || errMem != nil {
				// Handle cases where stats couldn't be retrieved (e.g., process exited mid-query)
				continue
			}

			nowTime := time.Now()

			// Update map
			if metrics, exists := processData[pid]; exists {
				// Process already tracked, update last seen stats and peak memory
				metrics.LastCPUTimes = cpuTimes
				if memInfo.RSS > metrics.PeakMemoryRSS {
					metrics.PeakMemoryRSS = memInfo.RSS
				}
				metrics.SampleCount++
				metrics.LastSeen = nowTime
			} else {
				// New process found
				processData[pid] = &ProcessMetrics{
					PID:             pid,
					Name:            name,
					Username:        username,
					InitialCPUTimes: cpuTimes,
					LastCPUTimes:    cpuTimes, // Initially, last is same as initial
					PeakMemoryRSS:   memInfo.RSS,
					SampleCount:     1,
					LastSeen:        nowTime,
				}
			}
		}

		// Optional: Clean up processes that disappeared between samples (more complex)
		// For simplicity here, we rely on the final calculation using Initial/Last seen values.
	}
	// --- End Monitoring Loop ---

	actualEndTime := time.Now()
	actualDuration := actualEndTime.Sub(startTime)

	// Get final *total* system CPU times
	finalSystemCPU, err := cpu.Times(false)
	if err != nil || len(finalSystemCPU) == 0 {
		log.Fatalf("Failed to get final system CPU stats: %v", err)
	}
	finalSystemTotalTime := getTotalCPUTime(&finalSystemCPU[0])

	// --- Calculate Final Stats ---
	systemTotalDelta := finalSystemTotalTime - initialSystemTotalTime
	if systemTotalDelta <= 0 {
		// Avoid division by zero or negative delta if time went backwards / system stats reset?
		log.Printf("Warning: System CPU time delta is non-positive (%f), CPU percentages might be inaccurate.", systemTotalDelta)
		systemTotalDelta = actualDuration.Seconds() // Fallback: use wall-clock time? Not ideal.
		if systemTotalDelta <= 0 { systemTotalDelta = 1 } // Prevent division by zero strictly
	}


	var cpuStats []ProcessCPUStat
	var memStats []ProcessMemStat

	for pid, metrics := range processData {
		// --- CPU Calculation ---
		// Ensure we have valid initial and last times
		if metrics.InitialCPUTimes == nil || metrics.LastCPUTimes == nil {
			log.Printf("Warning: Skipping CPU calculation for PID %d (%s) due to missing data.", pid, metrics.Name)
            continue
		}

		procInitialTime := getProcessCPUTime(metrics.InitialCPUTimes)
		procLastTime := getProcessCPUTime(metrics.LastCPUTimes)
		procDelta := procLastTime - procInitialTime

		// Average CPU % over the *entire monitoring duration*
		avgCPUPercent := (procDelta / systemTotalDelta) * 100.0
		// Clamp to 0-100 range, handle potential small negative due to precision/timing
		if avgCPUPercent < 0 { avgCPUPercent = 0 }
		if avgCPUPercent > 100 { avgCPUPercent = 100} // Clamp if delta logic flawed


		cpuStats = append(cpuStats, ProcessCPUStat{
			PID:         pid,
			Name:        metrics.Name,
			Username:    metrics.Username,
			AvgCPUPercent: avgCPUPercent,
		})

		// --- Memory Stat ---
		memStats = append(memStats, ProcessMemStat{
			PID:        pid,
			Name:       metrics.Name,
			Username:   metrics.Username,
			PeakRSS_KB: metrics.PeakMemoryRSS / 1024, // Convert bytes to KB
		})
	}

	// --- Sort Results ---
	// Sort by Avg CPU % (Descending)
	sort.Slice(cpuStats, func(i, j int) bool {
		return cpuStats[i].AvgCPUPercent > cpuStats[j].AvgCPUPercent
	})

	// Sort by Peak RSS (Descending)
	sort.Slice(memStats, func(i, j int) bool {
		return memStats[i].PeakRSS_KB > memStats[j].PeakRSS_KB
	})

	// --- Trim to Top N ---
	if len(cpuStats) > topN {
		cpuStats = cpuStats[:topN]
	}
	if len(memStats) > topN {
		memStats = memStats[:topN]
	}

	// --- Prepare Final Output ---
	result := Result{
		StartTime:       startTime,
		EndTime:         actualEndTime,
		Duration:        actualDuration.String(),
		TopCPUProcesses: cpuStats,
		TopMemProcesses: memStats,
	}

	// Marshal to JSON and print
	jsonData, err := json.MarshalIndent(result, "", "  ")
	if err != nil {
		log.Fatalf("Failed to marshal result to JSON: %v", err)
	}

	fmt.Println(string(jsonData))
}
