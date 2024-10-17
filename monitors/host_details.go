// host_details.go
// gets the host details that we need
// uptime, ip for now

package monitors

import (
    "fmt"
    "github.com/shirou/gopsutil/v3/host"
    "log"
    "net"

)

// Get preferred outbound ip of this machine
// does not make any connections
// https://stackoverflow.com/questions/23558425/how-do-i-get-the-local-ip-address-in-go
func getOutboundIP() net.IP {
    conn, err := net.Dial("udp", "8.8.8.8:80")
    if err != nil {
        log.Fatal(err)
    }
    defer conn.Close()

    localAddr := conn.LocalAddr().(*net.UDPAddr)

    
    return localAddr.IP
}

func GetHostDetails() (string, string, uint64, string, string, string) {

    /* What's in info
    {"hostname":"terra2","uptime":179922,"bootTime":1678566518,"procs":376,"os":"linux","platform":"fedora","platformFamily":"fedora","platformVersion":"37","kernelVersion":"6.1.11-200.fc37.x86_64","kernelArch":"x86_64","virtualizationSystem":"kvm","virtualizationRole":"host","hostId":"d49bd21c-0b92-4c1f-adc3-5e65b6a31c10"}
*/
    info, err := host.Info()
    if err != nil {
    fmt.Println(err)
    }
    //fmt.Println(info)

    ip := getOutboundIP()
    ipstring := ip.String()
    


    return info.HostID, info.Hostname, info.Uptime, info.OS, info.Platform,ipstring
    /*
    fmt.Printf("HostID: %v\n", info.HostID)
    fmt.Printf("Hostname: %v\n", info.Hostname)
    fmt.Printf("Uptime: %v\n", info.Uptime)
    fmt.Printf("OS: %v\n", info.OS)
    fmt.Printf("Platform: %v\n", info.Platform)
    fmt.Printf("IP: %v\n", getOutboundIP())
    */
}
