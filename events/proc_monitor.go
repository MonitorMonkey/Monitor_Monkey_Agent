package main

import (
	"bufio"
	"encoding/json"
	"flag"
	"fmt"
	"log"
	"os/exec"
	"strconv"
	"strings"
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
	RSS_KB   uint64 `json:"rss_kb"` // Renamed from PeakRSS_KB
}

// Result holds the final sorted lists for JSON output
type Result struct {
	TopCPUProcesses []ProcessCPUStat `json:"top_cpu_processes"`
	TopMemProcesses []ProcessMemStat `json:"top_mem_processes"`
}

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

	// Use 'axo' for specific fields and 'comm' for command name (less likely to have spaces than 'args')
	// Sorting is done by ps itself.
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
		// Command name might have spaces if 'comm' is longer than usual, but typically it's the last field here.
		// If 'comm' gets truncated and has spaces, this might need adjustment.
		// For simplicity, assume fields[4] is the command name.
		name := fields[4]
		if len(fields) > 5 {
			// If command name was split, rejoin it (basic attempt)
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
				RSS_KB:   rssKB, // ps 'rss' is already in KB
			})
		}
		count++
	}

	if err := scanner.Err(); err != nil {
		log.Printf("Error reading ps output: %v", err)
	}

	if err := cmd.Wait(); err != nil {
		// Ignore exit errors if we still got some data, ps might return error if some processes vanished
		if count == 0 {
			return nil, nil, fmt.Errorf("ps command failed: %w", err)
		}
		log.Printf("Warning: ps command finished with error (possibly ignorable): %v", err)
	}

	return cpuStats, memStats, nil
}


func main() {
	topNFlag := flag.Int("top", 10, "Number of top processes to report for CPU and Memory")
	flag.Parse()

	topN := *topNFlag
	if topN <= 0 {
		log.Fatalf("Invalid topN value: %d", topN)
	}

	log.Printf("Getting top %d processes by CPU and Memory using 'ps'", topN)

	// Get Top CPU Processes
	cpuStats, _, errCPU := getTopProcessesPS("cpu", topN)
	if errCPU != nil {
		log.Fatalf("Failed to get top CPU processes: %v", errCPU)
	}

	// Get Top Memory Processes
	_, memStats, errMem := getTopProcessesPS("mem", topN)
	if errMem != nil {
		log.Fatalf("Failed to get top Memory processes: %v", errMem)
	}

	// --- Prepare Final Output ---
	result := Result{
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
