// temp.go
// gets temperature of stuff

package monitors

import (
    "fmt"
    "github.com/shirou/gopsutil/v3/host"
)

type TemperatureReading struct {
    SensorKey    string
    Temperature  float64
}

func GetTemp() []TemperatureReading{

 	temp, err := host.SensorsTemperatures()
	if err != nil {
		fmt.Println(err)
	}
    readings := make([]TemperatureReading, 0)

    for _, temp := range temp {
       reading := TemperatureReading{SensorKey: temp.SensorKey, Temperature: temp.Temperature}
       readings = append(readings, reading)


        
    }
    return readings
    //fmt.Printf("type of temp is %t\n", readings)
    // Example on how to get specifc value (loop over it durrr)
    //fmt.Println(readings[0].SensorKey)
    //fmt.Println(readings[0].Temperature)
}
