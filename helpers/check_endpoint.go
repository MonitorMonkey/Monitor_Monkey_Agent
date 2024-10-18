package helpers

import (
    "fmt"
    "net"
    "net/url"
    "time"
)

func CheckEndpoint(endpoint string) bool {
    timeout := time.Second * 5

    parsedURL, err := url.Parse(endpoint)
    if err != nil {
        fmt.Printf("Could not parse URL %s: %s\n", endpoint, err)
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


    for {
        conn, err := net.DialTimeout("tcp", addr, timeout)
        if err != nil {
            fmt.Printf("Could not connect to %s: %s\n", addr, err)
            time.Sleep(time.Second * 5) // wait for 5 seconds before retrying
            continue
        }
        defer conn.Close()
        fmt.Printf("Successfully connected to %s\n", addr)
        return true
    }
}
