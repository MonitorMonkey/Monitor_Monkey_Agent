// agent version history
// 0.5 - net now excludes loopback
// 0.6.0 - now displays --version and --status to support upgrade script
// 0.6.1 - Memory optimisation
// 0.6.2 - Added open ports event monitoring
// 0.6.3 - Added process monitoring for CPU and Memory usage (in-memory storage)
// 0.6.4 - Sends Processs CPU/Mem on boot to populate frontend nicely
// 0.7.0 - Custom alerting
package main

import (
    "fmt"
    "go_monitor/monitors"
    "go_monitor/helpers"
    "go_monitor/events"
    "go_monitor/custom"
    "time"
    "encoding/json"
    "net/http"
    "bytes"
    "os"
    "io"
    "flag"
    "runtime/debug"
    "strconv"
)

// Version information
const AgentVersion = "0.7.0" 

type Custom struct {
    Disks []string
    Services []string
}

type mesure struct {
    Heartbeat int64
    Hostid string
    Hostname string
    Uptime uint64
    Os string
    Platform string
    Ip string
    Temp  []monitors.TemperatureReading
    Load  map[string]float64
    Disks map[string]float64
    Memory float64
    Upload uint64
    Download uint64
    UploadInterval uint64
    DownloadInterval uint64
    Services map[string]string
    AgentVer string
}

func log(to_log error) {
    fmt.Println(to_log)
}

// sendOpenPortsEvent gets open ports information and sends it to the events API
func sendOpenPortsEvent(client *http.Client, baseURL string, authHeader string) {
    // Get host ID and other details
    hostid, _, _, _, _, _ := monitors.GetHostDetails()
    
    // Get open ports data
    jsonData, err := events.GetOpenPortsJSON()
    if err != nil {
        fmt.Fprintf(os.Stderr, "Error getting open ports: %v\n", err)
        return
    }
    
    // Parse the JSON string back to a map for embedding in event data
    var portsData interface{}
    err = json.Unmarshal([]byte(jsonData), &portsData)
    if err != nil {
        fmt.Fprintf(os.Stderr, "Error parsing ports data: %v\n", err)
        return
    }
    
    // Create event payload
    eventPayload := map[string]interface{}{
        "Hostid":     hostid,
        "EventType":  "open_ports",
        "EventData":  portsData,
    }
    
    // Marshal the payload
    jsonBytes, err := json.Marshal(eventPayload)
    if err != nil {
        fmt.Fprintf(os.Stderr, "Error marshaling event data: %v\n", err)
        return
    }
    
    // Create and send the request
    eventsApi := baseURL + "/api/events/"
    req, err := http.NewRequest("POST", eventsApi, bytes.NewBuffer(jsonBytes))
    if err != nil {
        fmt.Fprintf(os.Stderr, "Error creating request: %v\n", err)
        return
    }
    
    req.Header.Set("Content-Type", "application/json")
    req.Header.Set("Authorization", authHeader)
    
    resp, err := client.Do(req)
    if err != nil {
        fmt.Fprintf(os.Stderr, "Error sending event: %v\n", err)
        return
    }
    defer resp.Body.Close()
    
    // Log success or failure
    if resp.StatusCode >= 200 && resp.StatusCode < 300 {
        fmt.Printf("Successfully sent open ports event at %s\n", time.Now().Format(time.RFC3339))
    } else {
        body, _ := io.ReadAll(resp.Body)
        fmt.Fprintf(os.Stderr, "Failed to send event. Status: %d, Response: %s\n", 
            resp.StatusCode, string(body))
    }
}

// sendProcessesEvent sends process data to the events API
func sendProcessesEvent(client *http.Client, baseURL string, authHeader string, metric string) {
    // Get host ID and other details
    hostid, _, _, _, _, _ := monitors.GetHostDetails()
    
    // Get the process data from memory
    jsonData, err := events.GetProcessesJSON(metric)
    if err != nil {
        fmt.Fprintf(os.Stderr, "Error getting %s processes data: %v\n", metric, err)
        return
    }
    
    // Parse the JSON string back to a map for embedding in event data
    var processData interface{}
    err = json.Unmarshal([]byte(jsonData), &processData)
    if err != nil {
        fmt.Fprintf(os.Stderr, "Error parsing %s processes data: %v\n", metric, err)
        return
    }
    
    // Create event payload
    eventType := ""
    if metric == "cpu" {
        eventType = "processes_cpu"
    } else if metric == "mem" {
        eventType = "processes_mem"
    } else {
        fmt.Fprintf(os.Stderr, "Invalid metric: %s\n", metric)
        return
    }
    
    eventPayload := map[string]interface{}{
        "Hostid":     hostid,
        "EventType":  eventType,
        "EventData":  processData,
    }
    
    // Marshal the payload
    jsonBytes, err := json.Marshal(eventPayload)
    if err != nil {
        fmt.Fprintf(os.Stderr, "Error marshaling event data: %v\n", err)
        return
    }
    
    // Create and send the request
    eventsApi := baseURL + "/api/events/"
    req, err := http.NewRequest("POST", eventsApi, bytes.NewBuffer(jsonBytes))
    if err != nil {
        fmt.Fprintf(os.Stderr, "Error creating request: %v\n", err)
        return
    }
    
    req.Header.Set("Content-Type", "application/json")
    req.Header.Set("Authorization", authHeader)
    
    resp, err := client.Do(req)
    if err != nil {
        fmt.Fprintf(os.Stderr, "Error sending event: %v\n", err)
        return
    }
    defer resp.Body.Close()
    
    // Log success or failure
    if resp.StatusCode >= 200 && resp.StatusCode < 300 {
        fmt.Printf("Successfully sent %s processes event at %s\n", metric, time.Now().Format(time.RFC3339))
    } else {
        body, _ := io.ReadAll(resp.Body)
        fmt.Fprintf(os.Stderr, "Failed to send %s processes event. Status: %d, Response: %s\n", 
            metric, resp.StatusCode, string(body))
    }
}

// sendProcessesEvents collects and sends both CPU and Memory process data to the events API
func sendProcessesEvents(client *http.Client, baseURL string, authHeader string) {
    // Send CPU processes
    sendProcessesEvent(client, baseURL, authHeader, "cpu")
    
    // Send Memory processes
    sendProcessesEvent(client, baseURL, authHeader, "mem")
    
    // Clear process data after sending to help with garbage collection
    events.ClearProcessData()
    
    // Force garbage collection
    debug.FreeOSMemory()
}

// collectProcessData collects process data on a regular schedule (more frequently than sending)
func collectProcessData(interval time.Duration, stopChan <-chan struct{}) {
    ticker := time.NewTicker(interval)
    defer ticker.Stop()
    
    // Collect initial data
    err := events.CollectProcesses(10) // Get top 10 processes
    if err != nil {
        fmt.Fprintf(os.Stderr, "Error in initial process data collection: %v\n", err)
    } else {
        fmt.Println("Initial process data collection completed")
    }
    
    for {
        select {
        case <-ticker.C:
            // Update process data periodically
            err := events.CollectProcesses(10) // Get top 10 processes
            if err != nil {
                fmt.Fprintf(os.Stderr, "Error collecting processes: %v\n", err)
            } else {
                fmt.Println("Process data updated at", time.Now().Format(time.RFC3339))
            }
        case <-stopChan:
            return
        }
    }
}

func main() {
    // Set up a recovery function to prevent crashes
    defer func() {
        if r := recover(); r != nil {
            fmt.Println("Recovered from panic:", r)
            fmt.Println(string(debug.Stack()))
            time.Sleep(time.Second * 10)
            main() // Restart the main function
        }
    }()

    // Parse command line arguments
    versionFlag := flag.Bool("version", false, "Display agent version")
    statusFlag := flag.Bool("status", false, "Display agent status")
    flag.Parse()

    // Handle version flag
    if *versionFlag {
        fmt.Printf("Monitor Monkey Agent version %s\n", AgentVersion)
        os.Exit(0)
    }

    // Handle status flag
    if *statusFlag {
        // Get host information
        hostid, hostname, uptime, osType, platform, ip := monitors.GetHostDetails()
        
        fmt.Println("Monitor Monkey Agent Status")
        fmt.Println("==========================")
        fmt.Printf("Version:  %s\n", AgentVersion)
        fmt.Printf("Hostname: %s\n", hostname)
        fmt.Printf("Host ID:  %s\n", hostid)
        fmt.Printf("IP:       %s\n", ip)
        fmt.Printf("OS:       %s %s\n", osType, platform)
        fmt.Printf("Uptime:   %d seconds\n", uptime)
        
        // Check if the service is running properly
        serviceStatus := monitors.ServiceCheck("monitor-monkey")
        fmt.Printf("Service:  %s\n", serviceStatus)
        
        os.Exit(0)
    }

    // Standard agent operation
    token := os.Getenv("MONKEY_API_KEY")
    if token == "" {
        fmt.Println("Error: MONKEY_API_KEY environment variable is not set")
        os.Exit(1)
    }
    
    authHeader := "token " + token
    //change
    const baseURL = "https://monitormonkey.io"
    //const baseURL = "http://192.168.1.131:8000"

    var (
        updateApi = baseURL + "/api/update/"
        confApi = baseURL + "/api/configure/"
    )

    // Create an HTTP client with timeout settings to prevent connection leaks
    client := &http.Client{
        Timeout: 30 * time.Second,
        Transport: &http.Transport{
            MaxIdleConns:        100,
            MaxIdleConnsPerHost: 20,
            IdleConnTimeout:     90 * time.Second,
            DisableKeepAlives:   false,
        },
    }

    // Set up process monitoring
    // Default collection interval: 5 minutes (to catch most significant activity)
    processCollectionInterval := 5 * time.Minute
    
    // Default sending interval: 24 hours
    processSendInterval := 24 * time.Hour
    
    // Environment variable overrides for development (now in seconds)
    if envInterval := os.Getenv("PROCESS_COLLECTION_INTERVAL"); envInterval != "" {
        if seconds, err := strconv.Atoi(envInterval); err == nil && seconds > 0 {
            processCollectionInterval = time.Duration(seconds) * time.Second
            fmt.Printf("Using custom process collection interval: %d seconds\n", seconds)
        }
    }
    
    if envInterval := os.Getenv("PROCESS_SEND_INTERVAL"); envInterval != "" {
        if seconds, err := strconv.Atoi(envInterval); err == nil && seconds > 0 {
            processSendInterval = time.Duration(seconds) * time.Second
            fmt.Printf("Using custom process send interval: %d seconds\n", seconds)
        }
    }
    
    fmt.Println("Process monitoring: Collection every", processCollectionInterval, "| Sending every", processSendInterval)
    fmt.Println("For testing, set PROCESS_COLLECTION_INTERVAL and PROCESS_SEND_INTERVAL env vars (in seconds)")
    
    // Start process data collection in a goroutine
    stopProcessCollection := make(chan struct{})
    go collectProcessData(processCollectionInterval, stopProcessCollection)
    
    // TODO:
    // Disks should be configured on agent boot for defaults
    // e.g just send all disks
    // then updates from api if need be.

    //defaultDisks := []string{"/", "/home"}
    defaultDisks := monitors.GetTopUsedDisks(2)
    defaultServices := []string{"sshd", "monitor-monkey"} // linux defaults again can be configured

    // Fetch configuration from API if it's configured
    // This is so we don't send 1 instance of non custom conf
    // Prepare the request payload with host details
    // Retrieve host details
    Hostid, Hostname, Uptime, Os, Platform, Ip := monitors.GetHostDetails()

    // Prepare the request payload with host details
    hostDetails := map[string]interface{}{
        "Hostid":   Hostid,
        "Hostname": Hostname,
        "Uptime":   Uptime,
        "Os":       Os,
        "Platform": Platform,
        "Ip":       Ip,
    }

    jsonPayload, err := json.Marshal(hostDetails)
    if err != nil {
        log(err)
    }

    // Create a POST request with host details
    req, err := http.NewRequest("POST", confApi, bytes.NewBuffer(jsonPayload))
    if err != nil {
        log(err)
    } else {
        req.Header.Set("Authorization", authHeader)
        req.Header.Set("Content-Type", "application/json")

        resp, err := client.Do(req)
        if err != nil {
            log(err)
        } else {
            defer resp.Body.Close()
            body, err := io.ReadAll(resp.Body)
            if err != nil {
                log(err)
            } else {
                var confResponse map[string]interface{}
                err = json.Unmarshal(body, &confResponse)
                if err != nil {
                    log(err)
                } else {
                    if value, ok := confResponse["message"]; ok && value == "noconf" {
                        fmt.Println("No configuration changes needed.")
                    } else {
                        var custom Custom
                        err = json.Unmarshal(body, &custom)
                        if err != nil {
                            log(err)
                        }
                        if custom.Disks != nil {
                            defaultDisks = custom.Disks
                        }
                        if custom.Services != nil {
                            defaultServices = custom.Services
                        }
                    }
                }
            }
        }
    }

    // Update interval
    interval := 5

    // Get initial network stats to establish a baseline
    initialUpload, initialDownload := monitors.GetNetStats()

    var oldUpload, oldDownload uint64 = 0, 0
    oldUpload, oldDownload = initialUpload, initialDownload

    fmt.Println("Initializing network monitoring... waiting for first interval")
    time.Sleep(time.Duration(interval) * time.Second)

    // Check endpoint with a controlled number of retries
    isAlive := false
    for i := 0; i < 3; i++ { // Limit retries to avoid resource exhaustion
        if helpers.CheckEndpoint(updateApi) {
            fmt.Println("The endpoint is alive")
            isAlive = true
            break
        }
        time.Sleep(time.Second * 5)
    }
    
    if !isAlive {
        fmt.Println("Warning: Endpoint check failed, but continuing operation")
    }
    
    // Force garbage collection before entering main loop
    debug.FreeOSMemory()
    
    // Set up monitoring intervals
    portsCheckInterval := 24 * time.Hour
    
    // Create tickers for periodic tasks
    portsTicker := time.NewTicker(portsCheckInterval)
    processesTicker := time.NewTicker(processSendInterval)
    
    // Initialize custom alerts monitor
    alertMonitor := custom.NewAlertMonitor(client, baseURL, authHeader, Hostid)
    alertMonitor.Start()
    
    // Run open ports check immediately once at startup
    go sendOpenPortsEvent(client, baseURL, authHeader)
    
    // Collect and send initial process data immediately at startup
    fmt.Println("Collecting initial process data...")
    err = events.CollectProcesses(10) // Collect top 10
    if err != nil {
        fmt.Fprintf(os.Stderr, "Error collecting initial process data: %v\n", err)
    } else {
        fmt.Println("Sending initial process data...")
        go sendProcessesEvents(client, baseURL, authHeader) // Send in a goroutine to avoid blocking startup
    }

    // Main monitoring loop
    for {
        // Create maps each iteration
        loadmap := make(map[string]float64)
        diskmap := make(map[string]float64)
        servicemap := make(map[string]string)

        m := mesure{}
        heartbeat := time.Now().Unix()
        m.Heartbeat = heartbeat

        m.Hostid, m.Hostname, m.Uptime, m.Os, m.Platform, m.Ip = monitors.GetHostDetails()
        m.Temp = monitors.GetTemp()
        m.Load = monitors.GetLoad(loadmap)

        for _, disk := range defaultDisks {
            diskmap[disk] = monitors.GetDiskUsage(disk)
        }
        m.Disks = diskmap
        m.Memory = monitors.GetMem()
        m.Upload, m.Download = monitors.GetNetStats()
        m.AgentVer = AgentVersion
        
        m.UploadInterval = m.Upload - oldUpload
        m.DownloadInterval = m.Download - oldDownload
        
        for _, service := range defaultServices {
            servicemap[service] = monitors.ServiceCheck(service)
        }
        m.Services = servicemap

        jsonBytes, err := json.Marshal(m)
        if err != nil {
            log(err)
            time.Sleep(time.Duration(interval) * time.Second)
            continue
        }

        // Create and send the request
        req, err := http.NewRequest("POST", updateApi, bytes.NewBuffer(jsonBytes))
        if err != nil {
            log(err)
            time.Sleep(time.Duration(interval) * time.Second)
            continue
        }
        
        req.Header.Set("Content-Type", "application/json")
        req.Header.Set("Authorization", authHeader)

        resp, err := client.Do(req)
        if err != nil {
            log(err)
            time.Sleep(time.Duration(interval) * time.Second)
            continue
        }
        
        // Always close the body to prevent resource leaks
        body, err := io.ReadAll(resp.Body)
        resp.Body.Close() // Explicitly close rather than defer to avoid accumulation

        if err != nil {
            log(err)
            time.Sleep(time.Duration(interval) * time.Second)
            continue
        }

        // Unmarshal into responseMap
        var responseMap map[string]interface{}
        err = json.Unmarshal(body, &responseMap)
        if err != nil {
            log(err)
            time.Sleep(time.Duration(interval) * time.Second)
            continue
        }

        // Check for "tomany" message
        if value, ok := responseMap["message"]; ok && value == "tomany" {
            fmt.Println("You have too many hosts being monitored for your payment plan")
            fmt.Println("Please remove some hosts or purchase some more :)")
            fmt.Println("I'll now go to sleep for a while 😪😪")
            time.Sleep(60 * time.Second)
        } else {
            // Unmarshal into custom struct
            var custom Custom
            err = json.Unmarshal(body, &custom)
            if err != nil {
                log(err)
            }
            if custom.Disks != nil {
                defaultDisks = custom.Disks
            }
            if custom.Services != nil {
                defaultServices = custom.Services
            }

            oldUpload = m.Upload
            oldDownload = m.Download

            // Explicitly clear out old data structures to help garbage collection
            body = nil
            jsonBytes = nil
            
            // Trigger garbage collection periodically
            if heartbeat % 60 == 0 {  // Every minute
                debug.FreeOSMemory()
            }
            
            // Check if it's time to send events (non-blocking)
            select {
            case <-portsTicker.C:
                go sendOpenPortsEvent(client, baseURL, authHeader)
            case <-processesTicker.C:
                go sendProcessesEvents(client, baseURL, authHeader)
            default:
                // Continue with the main loop
            }

            time.Sleep(time.Duration(interval) * time.Second)
        }
    }
}
