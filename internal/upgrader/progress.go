package upgrader

import "github.com/uozi-tech/cosy/logger"

type upgradeProgressWriter interface {
	WriteJSON(any) error
}

// startUpgradeProgress owns the progress receiver until the producer finishes.
func startUpgradeProgress(writer upgradeProgressWriter, message string) (chan float64, func()) {
	progress := make(chan float64)
	done := make(chan struct{})
	go func() {
		defer close(done)
		failed := false
		for value := range progress {
			// The upgrade still owns its producer after the browser disconnects.
			// Drain every subsequent value so an unbuffered send cannot stall it.
			if failed {
				continue
			}
			if err := writer.WriteJSON(CoreUpgradeResp{
				Status:   UpgradeStatusProgress,
				Progress: value,
				Message:  message,
			}); err != nil {
				logger.Error("Failed to send upgrade progress:", err)
				failed = true
			}
		}
	}()
	return progress, func() {
		close(progress)
		<-done
	}
}
