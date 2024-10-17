// mem.go
// gets memory

package monitors

import (
    "fmt"
    "github.com/shirou/gopsutil/v3/mem"
)

func GetMem() float64 {

 	memory, err := mem.VirtualMemory()
	if err != nil {
		fmt.Println(err)
	}

    //fmt.Println(load.Load1)
    return memory.UsedPercent
    // Example on how to get specifc value (loop over it durrr)
}
