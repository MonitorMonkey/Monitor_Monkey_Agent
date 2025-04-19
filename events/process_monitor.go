package events

import (
	"bufio"
	"encoding/json"
	"fmt"
	"log"
	"os/exec"
	"strconv"
	"strings"
	"sync"
)

// ProcessCPUStat holds formatted CPU results for JSON output
type ProcessCPUStat struct {
	PID         int32   `json:"pid"`
	Name        string  `json:"name"`
	Username    string  `json:"username"`
	CPUPercent  float64 `json:"cpu_percent"`
}

// ProcessMemStat holds formatted Memory results for JSON output
type ProcessMemStat struct {
	PID      int32  `json:"pid"`
	Name     string `json:"name"`
	Username string `json:"username"`
	RSS_KB   uint64 `json:"rss_kb"`
}

// Process data storage with mutex for thread safety
var (
	// Mutex for thread-safe access to process data
	processDataMutex sync.Mutex
	
	// In-memory storage for process data
	topCPUProcesses []ProcessCPUStat
	topMemProcesses []ProcessMemStat
)

// getTopProcessesPS executes the 'ps' command to get top processes sorted by CPU or Memory.
// metric: "cpu" or "mem"
// topN: number of processes to return
func getTopProcessesPS(metric string, topN int) ([]ProcessCPUStat, []ProcessMemStat, error) {
	var sortKey string
	switch metric {
	case "cpu":
		sortKey = "-%cpu"
	case "mem":
		sortKey = "-rss"
	default:
		return nil, nil, fmt.Errorf("invalid metric for sorting: %s", metric)
	}

	// Use 'axo' for specific fields and 'comm' for command name
	cmd := exec.Command("ps", "axo", "pid,user,%cpu,rss,comm", "--sort="+sortKey)
	stdout, err := cmd.StdoutPipe()
	if err != nil {
		return nil, nil, fmt.Errorf("failed to get stdout pipe: %w", err)
	}

	if err := cmd.Start(); err != nil {
		return nil, nil, fmt.Errorf("failed to start ps command: %w", err)
	}

	var cpuStats []ProcessCPUStat
	var memStats []ProcessMemStat
	count := 0

	scanner := bufio.NewScanner(stdout)
	scanner.Scan() // Skip header line

	for scanner.Scan() && count < topN {
		line := scanner.Text()
		fields := strings.Fields(line)
		if len(fields) < 5 {
			log.Printf("Warning: Skipping malformed ps line: %s", line)
			continue // Skip lines that don't have enough fields
		}

		pidStr := fields[0]
		user := fields[1]
		cpuPercentStr := fields[2]
		rssKBStr := fields[3]
		// Command name might have spaces if 'comm' is longer than usual
		name := fields[4]
		if len(fields) > 5 {
			// If command name was split, rejoin it
			name = strings.Join(fields[4:], " ")
		}

		pid, err := strconv.ParseInt(pidStr, 10, 32)
		if err != nil {
			log.Printf("Warning: Skipping line due to invalid PID '%s': %v", pidStr, err)
			continue
		}

		cpuPercent, err := strconv.ParseFloat(cpuPercentStr, 64)
		if err != nil {
			log.Printf("Warning: Skipping line due to invalid CPU%% '%s': %v", cpuPercentStr, err)
			continue
		}

		rssKB, err := strconv.ParseUint(rssKBStr, 10, 64)
		if err != nil {
			log.Printf("Warning: Skipping line due to invalid RSS '%s': %v", rssKBStr, err)
			continue
		}

		// Populate the correct slice based on the metric used for sorting
		if metric == "cpu" {
			cpuStats = append(cpuStats, ProcessCPUStat{
				PID:        int32(pid),
				Name:       name,
				Username:   user,
				CPUPercent: cpuPercent,
			})
		} else { // metric == "mem"
			memStats = append(memStats, ProcessMemStat{
				PID:      int32(pid),
				Name:     name,
				Username: user,
				RSS_KB:   rssKB,
			})
		}
		count++
	}

	if err := scanner.Err(); err != nil {
		log.Printf("Error reading ps output: %v", err)
	}

	if err := cmd.Wait(); err != nil {
		// Ignore exit errors if we still got some data, ps might return error if processes vanished
		if count == 0 {
			return nil, nil, fmt.Errorf("ps command failed: %w", err)
		}
		log.Printf("Warning: ps command finished with error (possibly ignorable): %v", err)
	}

	return cpuStats, memStats, nil
}

// CollectProcesses collects top processes and stores them in memory
func CollectProcesses(topN int) error {
    // Get current top CPU processes
    cpuStats, _, err := getTopProcessesPS("cpu", topN)
    if err != nil {
        return fmt.Errorf("failed to get top CPU processes: %w", err)
    }
    
    // Get current top Memory processes - THIS WAS MISSING!
    _, memStats, err := getTopProcessesPS("mem", topN)
    if err != nil {
        return fmt.Errorf("failed to get top Memory processes: %w", err)
    }
    
    // Update the in-memory storage with mutex lock for thread safety
    processDataMutex.Lock()
    defer processDataMutex.Unlock()
    
    // Replace existing data with new data
    topCPUProcesses = cpuStats
    topMemProcesses = memStats
    
    return nil
}

// GetProcessesJSON returns the current process data as JSON
func GetProcessesJSON(metric string) (string, error) {
	processDataMutex.Lock()
	defer processDataMutex.Unlock()
	
	if metric == "cpu" {
		if len(topCPUProcesses) == 0 {
			return "", fmt.Errorf("no CPU process data collected yet")
		}
		jsonData, err := json.Marshal(topCPUProcesses)
		if err != nil {
			return "", fmt.Errorf("failed to marshal CPU stats to JSON: %w", err)
		}
		return string(jsonData), nil
	} else if metric == "mem" {
		if len(topMemProcesses) == 0 {
			return "", fmt.Errorf("no Memory process data collected yet")
		}
		jsonData, err := json.Marshal(topMemProcesses)
		if err != nil {
			return "", fmt.Errorf("failed to marshal Memory stats to JSON: %w", err)
		}
		return string(jsonData), nil
	}
	
	return "", fmt.Errorf("invalid metric: %s", metric)
}

// ClearProcessData clears the in-memory process data after sending
// This helps with garbage collection
func ClearProcessData() {
	processDataMutex.Lock()
	defer processDataMutex.Unlock()
	
	// Clear slices to allow garbage collection
	topCPUProcesses = nil
	topMemProcesses = nil
}
