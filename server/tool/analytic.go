package tool

import (
    "fmt"
    "github.com/dustin/go-humanize"
    "github.com/minio/minio/pkg/disk"
    "strconv"
)

func DiskUsage(path string) (string, string, float64, error) {
    di, err := disk.GetInfo(path)
    if err != nil {
        return "", "", 0, err
    }
    percentage := (float64(di.Total-di.Free) / float64(di.Total)) * 100
    percentage, _ = strconv.ParseFloat(fmt.Sprintf("%.2f", percentage), 64)

    return humanize.Bytes(di.Total-di.Free), humanize.Bytes(di.Total),
        percentage, nil
}
