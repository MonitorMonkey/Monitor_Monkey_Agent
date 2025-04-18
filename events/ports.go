package events

import (
    "bufio"
    "encoding/hex"
    "encoding/json"
    "fmt"
    "net"
    "os"
    "sort"
    "strconv"
    "strings"
)

// PortInfo holds information about an open port.
type PortInfo struct {
    Port    int    `json:"port"`
    Service string `json:"service,omitempty"`
}

// OpenPorts holds the lists of open TCP and UDP ports.
type OpenPorts struct {
    TCP []PortInfo `json:"tcp"`
    UDP []PortInfo `json:"udp"`
}

// loadServices parses /etc/services to map ports to service names for TCP and UDP.
func loadServices() (map[int]string, map[int]string, error) {
    tcpServices := make(map[int]string)
    udpServices := make(map[int]string)

    file, err := os.Open("/etc/services")
    if err != nil {
        return tcpServices, udpServices, err // Return empty maps on error
    }
    defer file.Close()

    scanner := bufio.NewScanner(file)
    for scanner.Scan() {
        line := strings.TrimSpace(scanner.Text())
        if line == "" || strings.HasPrefix(line, "#") {
            continue
        }
        fields := strings.Fields(line)
        if len(fields) < 2 {
            continue
        }
        serviceName := fields[0]
        portProto := fields[1]
        parts := strings.Split(portProto, "/")
        if len(parts) != 2 {
            continue
        }
        portStr, proto := parts[0], parts[1]
        port, err := strconv.Atoi(portStr)
        if err != nil {
            continue
        }
        if proto == "tcp" {
            tcpServices[port] = serviceName
        } else if proto == "udp" {
            udpServices[port] = serviceName
        }
    }
    return tcpServices, udpServices, scanner.Err()
}

// parseLocalAddress converts a /proc/net/* local_address field into an IP and port.
func parseLocalAddress(addr string) (net.IP, int, error) {
    parts := strings.Split(addr, ":")
    if len(parts) != 2 {
        return nil, 0, fmt.Errorf("invalid address format: %s", addr)
    }
    ipHex, portHex := parts[0], parts[1]

    port, err := strconv.ParseUint(portHex, 16, 16)
    if err != nil {
        return nil, 0, fmt.Errorf("invalid port: %s", portHex)
    }

    if len(ipHex) == 8 { // IPv4
        ipBytes, err := hex.DecodeString(ipHex)
        if err != nil {
            return nil, 0, fmt.Errorf("invalid IP hex: %s", ipHex)
        }
        // Reverse bytes (little-endian in /proc/net/* to big-endian for net.IP)
        for i, j := 0, len(ipBytes)-1; i < j; i, j = i+1, j-1 {
            ipBytes[i], ipBytes[j] = ipBytes[j], ipBytes[i]
        }
        return net.IP(ipBytes), int(port), nil
    } else if len(ipHex) == 32 { // IPv6
        ipBytes, err := hex.DecodeString(ipHex)
        if err != nil {
            return nil, 0, fmt.Errorf("invalid IP hex: %s", ipHex)
        }
        return net.IP(ipBytes), int(port), nil
    }
    return nil, 0, fmt.Errorf("unsupported IP format: %s", ipHex)
}

// getOpenPorts reads a /proc/net/* file and returns a list of open ports.
func getOpenPorts(procFile string, isTCP bool) ([]int, error) {
    content, err := os.ReadFile(procFile)
    if err != nil {
        return nil, err // Return error if file cannot be read
    }

    lines := strings.Split(string(content), "\n")
    ports := make(map[int]bool) // Use map to collect unique ports
    for _, line := range lines[1:] { // Skip header line
        fields := strings.Fields(line)
        if len(fields) < 4 {
            continue
        }
        if isTCP && fields[3] != "0A" { // TCP: only LISTEN state (0A)
            continue
        }
        ip, port, err := parseLocalAddress(fields[1]) // local_address is 2nd field
        if err != nil {
            continue
        }
        if !ip.IsLoopback() { // Exclude loopback addresses
            ports[port] = true
        }
    }
    var portList []int
    for port := range ports {
        portList = append(portList, port)
    }
    return portList, nil
}

// GetOpenPorts returns information about open TCP and UDP ports.
func GetOpenPorts() (OpenPorts, error) {
    // Load service mappings from /etc/services
    tcpServices, udpServices, err := loadServices()
    if err != nil {
        fmt.Fprintf(os.Stderr, "Warning: could not load /etc/services: %v\n", err)
    }

    // Collect unique open ports for TCP and UDP
    tcpPorts := make(map[int]bool)
    udpPorts := make(map[int]bool)

    // Process TCP files (IPv4 and IPv6)
    for _, file := range []string{"/proc/net/tcp", "/proc/net/tcp6"} {
        ports, err := getOpenPorts(file, true)
        if err != nil {
            fmt.Fprintf(os.Stderr, "Error reading %s: %v\n", file, err)
            continue
        }
        for _, port := range ports {
            tcpPorts[port] = true
        }
    }

    // Process UDP files (IPv4 and IPv6)
    for _, file := range []string{"/proc/net/udp", "/proc/net/udp6"} {
        ports, err := getOpenPorts(file, false)
        if err != nil {
            fmt.Fprintf(os.Stderr, "Error reading %s: %v\n", file, err)
            continue
        }
        for _, port := range ports {
            udpPorts[port] = true
        }
    }

    // Create slices of PortInfo for TCP and UDP
    var tcpPortInfos []PortInfo
    for port := range tcpPorts {
        service, ok := tcpServices[port]
        if ok {
            tcpPortInfos = append(tcpPortInfos, PortInfo{Port: port, Service: service})
        } else {
            tcpPortInfos = append(tcpPortInfos, PortInfo{Port: port})
        }
    }

    var udpPortInfos []PortInfo
    for port := range udpPorts {
        service, ok := udpServices[port]
        if ok {
            udpPortInfos = append(udpPortInfos, PortInfo{Port: port, Service: service})
        } else {
            udpPortInfos = append(udpPortInfos, PortInfo{Port: port})
        }
    }

    // Sort the port info slices by port number
    sort.Slice(tcpPortInfos, func(i, j int) bool {
        return tcpPortInfos[i].Port < tcpPortInfos[j].Port
    })
    sort.Slice(udpPortInfos, func(i, j int) bool {
        return udpPortInfos[i].Port < udpPortInfos[j].Port
    })

    // Create the OpenPorts struct
    openPorts := OpenPorts{
        TCP: tcpPortInfos,
        UDP: udpPortInfos,
    }

    return openPorts, nil
}

// GetOpenPortsJSON returns open ports information as a formatted JSON string
func GetOpenPortsJSON() (string, error) {
    openPorts, err := GetOpenPorts()
    if err != nil {
        return "", err
    }

    // Marshal the struct to JSON
    jsonData, err := json.MarshalIndent(openPorts, "", "  ")
    if err != nil {
        return "", err
    }

    return string(jsonData), nil
}
