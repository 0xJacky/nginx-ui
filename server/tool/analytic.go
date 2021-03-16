package tool

import (
    "fmt"
    "github.com/dustin/go-humanize"
    "github.com/minio/minio/pkg/disk"
    "strconv"
)

func DiskUsage(path string) (string, string, string, error) {
    di, err := disk.GetInfo(path)
    if err != nil {
        return "", "", "", err
    }
    percentage := (float64(di.Total-di.Free) / float64(di.Total)) * 100
    fmt.Printf("%s of %s disk space used (%0.2f%%)\n",
        humanize.Bytes(di.Total-di.Free),
        humanize.Bytes(di.Total),
        percentage,
    )
    return humanize.Bytes(di.Total-di.Free), humanize.Bytes(di.Total),
        strconv.FormatFloat(percentage, 'f', 2, 64), nil
}
