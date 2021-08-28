package test

import (
	"fmt"
	humanize "github.com/dustin/go-humanize"
	"github.com/mackerelio/go-osstat/cpu"
	"github.com/mackerelio/go-osstat/disk"
	"github.com/mackerelio/go-osstat/memory"
	"os"
	"runtime"
	"testing"
	"time"
)

func TestGetArch(t *testing.T) {
	fmt.Println("os:", runtime.GOOS)
	fmt.Println("threads:", runtime.GOMAXPROCS(0))

	memoryStat, err := memory.Get()
	if err != nil {
		fmt.Fprintf(os.Stderr, "%s\n", err)
		return
	}
	fmt.Println("memory total:", humanize.Bytes(memoryStat.Total))
	fmt.Println("memory used:", humanize.Bytes(memoryStat.Used))
	fmt.Println("memory cached:", humanize.Bytes(memoryStat.Cached))
	fmt.Println("memory free:", humanize.Bytes(memoryStat.Free))

	before, err := cpu.Get()
	if err != nil {
		fmt.Println(err)
	}
	time.Sleep(time.Duration(1) * time.Second)
	after, err := cpu.Get()
	if err != nil {
		fmt.Println(err)
	}
	total := float64(after.Total - before.Total)
	fmt.Printf("cpu user: %f %%\n", float64(after.User-before.User)/total*100)
	fmt.Printf("cpu system: %f %%\n", float64(after.System-before.System)/total*100)
	fmt.Printf("cpu idle: %f %%\n", float64(after.Idle-before.Idle)/total*100)

	err = diskUsage(".")

	if err != nil {
		fmt.Println(err)
	}
}

func diskUsage(path string) error {
	di, err := disk.GetInfo(path)
	if err != nil {
		return err
	}
	percentage := (float64(di.Total-di.Free) / float64(di.Total)) * 100
	fmt.Printf("%s of %s disk space used (%0.2f%%)\n",
		humanize.Bytes(di.Total-di.Free),
		humanize.Bytes(di.Total),
		percentage,
	)
	return nil
}
