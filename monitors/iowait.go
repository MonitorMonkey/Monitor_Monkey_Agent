package monitors

import (
	"fmt"
    "syscall"
)

func GetIOWait() {
    var info syscall.Sysinfo_t
    if err := syscall.Sysinfo(&info); err != nil {
        panic(err)
    }
    fmt.Printf("IOWait: %d\n", info.Uptime)
}

