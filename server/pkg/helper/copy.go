package helper

import (
    "io"
    "os"
)

func CopyFile(src, dst string) (int64, error) {
    sourceFileStat, err := os.Stat(src)
    if err != nil {
        return 0, err
    }

    if !sourceFileStat.Mode().IsRegular() {
        return 0, nil
    }

    source, err := os.Open(src)
    if err != nil {
        return 0, err
    }
    defer source.Close()

    destination, err := os.Create(dst)
    if err != nil {
        return 0, err
    }
    defer destination.Close()

    return io.Copy(destination, source)
}
