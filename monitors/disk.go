// disk.go
// gets disk info

package monitors

import (
    "fmt"
    "github.com/shirou/gopsutil/v3/disk"
)


/*

func GetDefaultDisks(amount int) []uint64 {
    // get list of all disks ✅
    // figure out total space of all disks
    // figure out the top X by space
    // return them
    parts, err := disk.Partitions(false)
    if err != nil {
        fmt.Println(err)
    }
    fmt.Println(parts)

    var diskSizes []uint64
    // init the slice to hold our largest disks
    largestDisks := make([]uint64, amount)
    

    for _, disk := range parts {
        path := disk.Mountpoint
        diskSizes = append(diskSizes, GetDiskSize(path))
    }


    for _, size := range diskSizes {
        // if the current number is larger than the smallest largest number,
        // replace the smallest largest number with the current number
        if size > largestDisks[0] {
            largestDisks[0] = size
            // sort the largest slice in descending order
        //    sort.Sort(sort.Reverse(sort.IntSlice(largestDisks)))
        }
    }
    return largestDisks

}
*/



func GetDiskUsage(diskPath string) float64 {

    diskStat, err := disk.Usage(diskPath)
    if err != nil {
        fmt.Println(err)
        // we dont' want this to crash if we don't have a disk so we just return 0
        return 0.0
    }
    return diskStat.UsedPercent
}

func GetDiskSize(diskPath string) uint64 {

    diskStat, err := disk.Usage(diskPath)
    if err != nil {
        fmt.Println(err)
    }
    return diskStat.Total
}

