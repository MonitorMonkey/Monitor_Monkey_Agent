package custom

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"sync"
	"time"
)

// Default directory to look for .mm files
const DefaultAlertsDir = "/opt/monitor-monkey/custom-events/"

// Environment variable name to override the default directory
const AlertsDirEnvVar = "MONKEY_CUSTOM_ALERTS_DIR"

// Minimum allowed interval for alerts (1 minute)
const MinAlertInterval = time.Minute

// AlertDefinition represents a parsed .mm file
type AlertDefinition struct {
	Path     string    // Path to the .mm file
	Name     string    // Alert name
	Interval time.Duration // Alert interval
	Data     interface{} // Alert data (can be string, number, etc.)
	LastSent time.Time // Last time this alert was sent
}

// AlertMonitor handles the custom alerts functionality
type AlertMonitor struct {
	alertsDir     string
	alerts        map[string]*AlertDefinition
	client        *http.Client
	baseURL       string
	authHeader    string
	hostID        string
	stopChan      chan struct{}
	mutex         sync.Mutex
}

// NewAlertMonitor creates a new alert monitor instance
func NewAlertMonitor(client *http.Client, baseURL, authHeader, hostID string) *AlertMonitor {
	// Get alerts directory from environment variable or use default
	alertsDir := os.Getenv(AlertsDirEnvVar)
	if alertsDir == "" {
		alertsDir = DefaultAlertsDir
	}

	return &AlertMonitor{
		alertsDir:  alertsDir,
		alerts:     make(map[string]*AlertDefinition),
		client:     client,
		baseURL:    baseURL,
		authHeader: authHeader,
		hostID:     hostID,
		stopChan:   make(chan struct{}),
		mutex:      sync.Mutex{},
	}
}

// Start begins monitoring and sending custom alerts
func (am *AlertMonitor) Start() {
	fmt.Println("Starting custom alerts monitor")
	
	// Create alerts directory if it doesn't exist
	if err := os.MkdirAll(am.alertsDir, 0755); err != nil {
		fmt.Fprintf(os.Stderr, "Error creating alerts directory: %v\n", err)
	}
	
	// Load initial alerts
	am.loadAlerts()
	
	// Send all alerts immediately on boot
	fmt.Println("Sending all custom alerts immediately on startup")
	am.sendAllAlerts()
	
	// Start the monitoring goroutine
	go am.monitorAlerts()
}

// sendAllAlerts sends all loaded alerts immediately
func (am *AlertMonitor) sendAllAlerts() {
	am.mutex.Lock()
	alertCount := len(am.alerts)
	am.mutex.Unlock()
	
	if alertCount == 0 {
		fmt.Println("No custom alerts found to send")
		return
	}
	
	fmt.Printf("Sending %d custom alerts...\n", alertCount)
	
	am.mutex.Lock()
	for _, alert := range am.alerts {
		go am.sendAlert(alert)
		alert.LastSent = time.Now() // Update the last sent time
	}
	am.mutex.Unlock()
}

// Stop stops the alert monitoring
func (am *AlertMonitor) Stop() {
	close(am.stopChan)
}

// loadAlerts scans the alerts directory and loads all .mm files
func (am *AlertMonitor) loadAlerts() {
	fmt.Printf("Loading alerts from %s\n", am.alertsDir)
	
	// Scan directory for .mm files
	files, err := filepath.Glob(filepath.Join(am.alertsDir, "*.mm"))
	if err != nil {
		fmt.Fprintf(os.Stderr, "Error scanning alerts directory: %v\n", err)
		return
	}
	
	// Store existing timestamps before we reset the alerts map
	lastSentTimes := make(map[string]time.Time)
	am.mutex.Lock()
	for path, alert := range am.alerts {
		lastSentTimes[path] = alert.LastSent
	}
	
	// Create new alerts map but don't immediately assign it
	newAlerts := make(map[string]*AlertDefinition)
	am.mutex.Unlock()
	
	// Process each file
	for _, file := range files {
		if alert, err := am.parseAlertFile(file); err != nil {
			fmt.Fprintf(os.Stderr, "Error parsing alert file %s: %v\n", file, err)
		} else {
			// Preserve the LastSent timestamp if this alert existed before
			if lastSent, exists := lastSentTimes[file]; exists {
				alert.LastSent = lastSent
			}
			
			newAlerts[file] = alert
			fmt.Printf("Loaded alert: %s, interval: %v\n", alert.Name, alert.Interval)
		}
	}
	
	// Update the alerts map with the new version
	am.mutex.Lock()
	am.alerts = newAlerts
	am.mutex.Unlock()
}

// parseAlertFile reads and parses a .mm file
func (am *AlertMonitor) parseAlertFile(path string) (*AlertDefinition, error) {
	// Read file content
	content, err := os.ReadFile(path)
	if err != nil {
		return nil, fmt.Errorf("failed to read file: %w", err)
	}
	
	// Parse the content
	contentStr := string(content)
	lines := strings.Split(contentStr, "\n")
	
	alert := &AlertDefinition{
		Path: path,
	}
	
	// Parse each line (expecting key=value format)
	for i, line := range lines {
		// Skip empty lines
		line = strings.TrimSpace(line)
		if line == "" {
			continue
		}
		
		// Skip comment lines
		if strings.HasPrefix(line, "#") {
			continue
		}
		
		// Only process the first three non-empty lines for metadata
		if i >= 3 && alert.Name != "" && alert.Interval != 0 && alert.Data != nil {
			break
		}
		
		kv := strings.SplitN(line, "=", 2)
		if len(kv) != 2 {
			continue
		}
		
		key := strings.TrimSpace(kv[0])
		valueRaw := strings.TrimSpace(kv[1])
		
		switch key {
		case "name":
			// Remove surrounding quotes if present
			alert.Name = strings.Trim(valueRaw, "\"")
		case "interval":
			// Remove surrounding quotes if present
			intervalStr := strings.Trim(valueRaw, "\"")
			// Parse interval
			interval, err := parseInterval(intervalStr)
			if err != nil {
				return nil, fmt.Errorf("invalid interval format: %w", err)
			}
			// Ensure minimum interval
			if interval < MinAlertInterval {
				interval = MinAlertInterval
				fmt.Printf("Warning: Interval for %s is less than minimum, using %v instead\n", path, MinAlertInterval)
			}
			alert.Interval = interval
		case "data":
			// Check if it's a quoted string
			if strings.HasPrefix(valueRaw, "\"") && strings.HasSuffix(valueRaw, "\"") {
				// String value
				alert.Data = strings.Trim(valueRaw, "\"")
			} else {
				// Try to parse as number
				if intVal, err := strconv.Atoi(valueRaw); err == nil {
					alert.Data = intVal
				} else if floatVal, err := strconv.ParseFloat(valueRaw, 64); err == nil {
					alert.Data = floatVal
				} else {
					// If parsing fails, use as string
					alert.Data = valueRaw
				}
			}
		}
	}
	
	// Validate required fields
	if alert.Name == "" {
		return nil, fmt.Errorf("missing name field")
	}
	if alert.Interval == 0 {
		return nil, fmt.Errorf("missing or invalid interval field")
	}
	if alert.Data == nil {
		return nil, fmt.Errorf("missing data field")
	}
	
	return alert, nil
}

// parseInterval converts interval strings like "1hr", "30m" to time.Duration
func parseInterval(interval string) (time.Duration, error) {
	// Common interval formats
	if strings.HasSuffix(interval, "ms") {
		val := strings.TrimSuffix(interval, "ms")
		if ms, err := time.ParseDuration(val + "ms"); err == nil {
			return ms, nil
		}
	} else if strings.HasSuffix(interval, "s") {
		val := strings.TrimSuffix(interval, "s")
		if s, err := time.ParseDuration(val + "s"); err == nil {
			return s, nil
		}
	} else if strings.HasSuffix(interval, "m") {
		val := strings.TrimSuffix(interval, "m")
		if m, err := time.ParseDuration(val + "m"); err == nil {
			return m, nil
		}
	} else if strings.HasSuffix(interval, "h") || strings.HasSuffix(interval, "hr") || strings.HasSuffix(interval, "hrs") {
		val := strings.TrimSuffix(strings.TrimSuffix(strings.TrimSuffix(interval, "hrs"), "hr"), "h")
		if h, err := time.ParseDuration(val + "h"); err == nil {
			return h, nil
		}
	} else if strings.HasSuffix(interval, "d") {
		val := strings.TrimSuffix(interval, "d")
		if i, err := parseInt(val); err == nil {
			return time.Duration(i) * 24 * time.Hour, nil
		}
	}
	
	// Try standard Go duration format
	return time.ParseDuration(interval)
}

// parseInt converts a string to int, with error handling
func parseInt(val string) (int, error) {
	var result int
	_, err := fmt.Sscanf(val, "%d", &result)
	return result, err
}

// monitorAlerts periodically checks and sends alerts
func (am *AlertMonitor) monitorAlerts() {
	// Ticker for checking alerts (every minute)
	ticker := time.NewTicker(MinAlertInterval)
	defer ticker.Stop()
	
	for {
		select {
		case <-ticker.C:
			am.checkAlerts()
		case <-am.stopChan:
			return
		}
	}
}

// checkAlerts looks through all alerts to see which ones need to be sent
func (am *AlertMonitor) checkAlerts() {
	// Reload alerts to pick up any changes
	am.loadAlerts()
	
	now := time.Now()
	
	am.mutex.Lock()
	defer am.mutex.Unlock()
	
	for _, alert := range am.alerts {
		// Check if it's time to send this alert
		if now.Sub(alert.LastSent) >= alert.Interval {
			go am.sendAlert(alert)
			alert.LastSent = now
		}
	}
}

// sendAlert sends an alert to the custom-events API
func (am *AlertMonitor) sendAlert(alert *AlertDefinition) {
	// Create event payload in the format expected by custom-events endpoint
	eventPayload := map[string]interface{}{
		"host_id": am.hostID,
		"name": alert.Name,
		"value": alert.Data,
	}
	
	// Marshal the payload
	jsonBytes, err := json.Marshal(eventPayload)
	if err != nil {
		fmt.Fprintf(os.Stderr, "Error marshaling alert data: %v\n", err)
		return
	}
	
	// Create and send the request to the custom-events endpoint
	customEventsApi := am.baseURL + "/api/custom-events/"
	req, err := http.NewRequest("POST", customEventsApi, bytes.NewBuffer(jsonBytes))
	if err != nil {
		fmt.Fprintf(os.Stderr, "Error creating request: %v\n", err)
		return
	}
	
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Authorization", am.authHeader)
	
	resp, err := am.client.Do(req)
	if err != nil {
		fmt.Fprintf(os.Stderr, "Error sending alert: %v\n", err)
		return
	}
	defer resp.Body.Close()
	
	// Log success or failure
	if resp.StatusCode >= 200 && resp.StatusCode < 300 {
		fmt.Printf("Successfully sent custom alert '%s' at %s\n", alert.Name, time.Now().Format(time.RFC3339))
	} else {
		body, _ := io.ReadAll(resp.Body)
		fmt.Fprintf(os.Stderr, "Failed to send alert '%s'. Status: %d, Response: %s\n", 
			alert.Name, resp.StatusCode, string(body))
	}
}
