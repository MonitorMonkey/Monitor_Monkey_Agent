
// temp.go
// gets temperature of stuff

package monitors

import (
    "fmt"
    "github.com/shirou/gopsutil/v3/load"
)

func GetLoad(loadmap map[string]float64) map[string]float64 {

 	load, err := load.Avg()
	if err != nil {
		fmt.Println(err)
	}

    //fmt.Println(load)
    //return load
    // Example on how to get specifc value (loop over it durrr)
    
    // TODO:
    // DO something for windows
    loadmap["load1"] = load.Load1
    loadmap["load5"] = load.Load5
    loadmap["load15"] = load.Load15
    return loadmap
}
/*
func GetPs() {

 	ps, err := load.Misc()
	if err != nil {
		fmt.Println(err)
	}

    //fmt.Println(load.Load1)
    fmt.Println(ps)
    // Example on how to get specifc value (loop over it durrr)
}
*/
