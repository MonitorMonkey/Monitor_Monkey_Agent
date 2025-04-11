package helpers

import (
    "fmt"
    "net"
    "net/url"
    "time"
)

func CheckEndpoint(endpoint string) bool {
    timeout := time.Second * 5
    maxRetries := 3  // Limit retries to prevent resource exhaustion

    parsedURL, err := url.Parse(endpoint)
    if err != nil {
        fmt.Printf("Could not parse URL %s: %s\n", endpoint, err)
        return false
    } 

    if parsedURL.Port() == "" {
        if parsedURL.Scheme == "http" {
            parsedURL.Host += ":80"
        } else if parsedURL.Scheme == "https" {
            parsedURL.Host += ":443"
        }
    }

    addr := net.JoinHostPort(parsedURL.Hostname(), parsedURL.Port())
    fmt.Println(addr)

    for i := 0; i < maxRetries; i++ {
        conn, err := net.DialTimeout("tcp", addr, timeout)
        if err != nil {
            fmt.Printf("Could not connect to %s: %s (attempt %d/%d)\n", addr, err, i+1, maxRetries)
            if i < maxRetries-1 {
                // Exponential backoff
                sleepTime := time.Duration(1<<uint(i)) * time.Second
                if sleepTime > 15*time.Second {
                    sleepTime = 15 * time.Second
                }
                fmt.Printf("Waiting %v before retrying...\n", sleepTime)
                time.Sleep(sleepTime)
            }
            continue
        }
        
        conn.Close()  // Explicitly close the connection
        fmt.Printf("Successfully connected to %s\n", addr)
        return true
    }
    
    fmt.Printf("Failed to connect to %s after %d attempts\n", addr, maxRetries)
    return false
}
