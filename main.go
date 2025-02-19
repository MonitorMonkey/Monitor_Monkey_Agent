package main

import (
    "fmt"
     "go_monitor/monitors"
     "go_monitor/helpers"
     "time"
     "encoding/json"
     "net/http"
     "bytes"
     "os"
     "io"
)



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
    /*
    DEBUG := true
    syslog, err := syslog.New(syslog.LOG_ERR, "monit")
    if err != nil {
        fmt.Println("Unable to connect to syslog daemon")
    }
    defer syslog.Close()

    if DEBUG == true {
        fmt.Println(to_log)
    } else {
        syslog.Err(to_log.Error())
    }
    */
}
        
    


func main() {

    // false will log to syslog, true will print to console


    token := os.Getenv("MONKEY_API_KEY")
	if token == "" {
		fmt.Println("Error: API_KEY environment variable is not set")
		os.Exit(1)
	}
	AgentVer := "0.3"
    authHeader := "token " + token
	//change
	const baseURL = "https://monitormonkey.io"
	//const baseURL = "http://192.168.1.172:8000"

	var (
		updateApi  = baseURL + "/api/update/"
		confApi = baseURL + "/api/configure/"
	)

    client := &http.Client{}


    // TODO:
    // Disks should be configured on agent boot for defaults
	// e.g just send all disks
	// then updates from api if need be.

    //defaultDisks := []string{"/", "/home"}
    defaultDisks := monitors.GetTopUsedDisks(2)
    defaultServices := []string{"sshd", "httpd"} // liunx defaults again can be configured


    
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
	}
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


	// update interval
    interval := 5

    var oldUpload, oldDownload uint64 = 0, 0

    if helpers.CheckEndpoint(updateApi) == true {
        fmt.Println("the endpoint is alive")
    }

    for {

        loadmap := make(map[string]float64)
        diskmap := make(map[string]float64)
        servicemap := make(map[string]string)


        

        m := mesure{}
        heartbeat := time.Now().Unix()
        m.Heartbeat = heartbeat
        //fmt.Printf("Heartbeat (unix)time %v\n", time.Now().Unix())

        m.Hostid, m.Hostname, m.Uptime, m.Os, m.Platform, m.Ip = monitors.GetHostDetails()

        m.Temp = monitors.GetTemp()

        m.Load = monitors.GetLoad(loadmap)

        for _, disk := range defaultDisks {
            diskmap[disk] = monitors.GetDiskUsage(disk)
        }
        m.Disks = diskmap
        m.Memory = monitors.GetMem()
        m.Upload, m.Download = monitors.GetNetStats()
		m.AgentVer = AgentVer
        // TODO:
        // should do the caculation (new up - old up)
        // to get amount sent in given timeframe
        // but that does not want to work
        // so will do it api side for now
        m.UploadInterval = m.Upload - oldUpload
        m.DownloadInterval = m.Download - oldDownload
        for _, service := range defaultServices {
            //fmt.Printf("Service %v %v\n", service, monitors.ServiceCheck(service))
            servicemap[service] = monitors.ServiceCheck(service)

        }
        m.Services = servicemap
        // TODO:
        // Implement 
        //monitors.GetIOWait()



        jsonBytes, err := json.Marshal(m)

        if err != nil {
            log(err)
        }
        //fmt.Println(string(jsonBytes))

        req, err := http.NewRequest("POST", updateApi, bytes.NewBuffer(jsonBytes))
        req.Header.Set("Content-Type", "application/json")
        req.Header.Set("Content-Type", "application/json")
        req.Header.Set("Authorization", authHeader)

        resp, err := client.Do(req)
        if err != nil {
            log(err)
            // TODO: if panic here, just sleep and try again (if api serv is down)
			//TODO: Test if panic is fine, systemd can deal with it.
        }
        defer resp.Body.Close()

		// Read the response body
		body, err := io.ReadAll(resp.Body)
		if err != nil {
			panic(err)
		}
		defer resp.Body.Close()

		// Unmarshal into responseMap
		var responseMap map[string]interface{}
		err = json.Unmarshal(body, &responseMap)
		if err != nil {
			panic(err)
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

			//fmt.Println("ran successful")
        	oldUpload = m.Upload
        	oldDownload = m.Download




        	time.Sleep(time.Duration(interval) * time.Second)
		}


    }


}

