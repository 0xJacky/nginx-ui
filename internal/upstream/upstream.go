package upstream

import (
	"net"
	"sync"
	"time"
)

const MaxTimeout = 5 * time.Second
const MaxConcurrentWorker = 10

type Status struct {
	Online  bool    `json:"online"`
	Latency float32 `json:"latency"`
}

func AvailabilityTest(body []string) (result map[string]*Status) {
	result = make(map[string]*Status)

	wg := sync.WaitGroup{}
	wg.Add(len(body))
	c := make(chan struct{}, MaxConcurrentWorker)
	for _, socket := range body {
		c <- struct{}{}
		s := &Status{}
		go testLatency(c, &wg, socket, s)
		result[socket] = s
	}
	wg.Wait()

	return
}

func testLatency(c chan struct{}, wg *sync.WaitGroup, socket string, status *Status) {
	defer func() {
		wg.Done()
		<-c
	}()

	scopedWg := sync.WaitGroup{}
	scopedWg.Add(2)
	go testTCPLatency(&scopedWg, socket, status)
	go testUnixSocketLatency(&scopedWg, socket, status)
	scopedWg.Wait()
}

func testTCPLatency(wg *sync.WaitGroup, socket string, status *Status) {
	defer func() {
		wg.Done()
	}()
	start := time.Now()
	conn, err := net.DialTimeout("tcp", socket, MaxTimeout)

	if err != nil {
		return
	}

	defer conn.Close()

	end := time.Now()

	status.Online = true
	status.Latency = float32(end.Sub(start)) / float32(time.Millisecond)
}

func testUnixSocketLatency(wg *sync.WaitGroup, socket string, status *Status) {
	defer func() {
		wg.Done()
	}()
	start := time.Now()
	conn, err := net.DialTimeout("unix", socket, MaxTimeout)

	if err != nil {
		return
	}

	defer conn.Close()

	end := time.Now()

	status.Online = true
	status.Latency = float32(end.Sub(start)) / float32(time.Millisecond)
}
