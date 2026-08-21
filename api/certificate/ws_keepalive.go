package certificate

import (
	"context"
	"time"

	"github.com/gorilla/websocket"
)

const (
	issueCertWSWriteWait  = 10 * time.Second
	issueCertWSPingPeriod = 20 * time.Second
)

type issueCertControlWriter interface {
	WriteControl(messageType int, data []byte, deadline time.Time) error
	Close() error
}

func startIssueCertKeepalive(writer issueCertControlWriter, period time.Duration) context.CancelFunc {
	ctx, cancel := context.WithCancel(context.Background())

	go func() {
		ticker := time.NewTicker(period)
		defer ticker.Stop()

		for {
			select {
			case <-ctx.Done():
				return
			case <-ticker.C:
				if err := writer.WriteControl(websocket.PingMessage, nil, time.Now().Add(issueCertWSWriteWait)); err != nil {
					_ = writer.Close()
					return
				}
			}
		}
	}()

	return cancel
}
