package monitors

import (
    "fmt"
    "github.com/shirou/gopsutil/v3/net"
)


func GetNetStats() (uint64, uint64) {
    nstats, err := net.IOCounters(false)
    if err != nil {
        fmt.Println(err)
    }
    /*
   new_upload := nstats[0].BytesSent
   new_download := nstats[0].BytesRecv
   fmt.Printf("Old upload is %v, new upload is %v\n", old_upload, new_upload)

   current_upload := new_upload - old_upload
   current_download := new_download - old_download

   return current_upload, current_download
   */
   total_upload := nstats[0].BytesSent
   total_download := nstats[0].BytesRecv

   return total_upload, total_download



}

/*
func GetNetStats() {
    nstats, err := net.IOCounters(false)
    if err != nil {
        fmt.Println(err)
    }
    fmt.Println(nstats[0].BytesSent)
}
*/
