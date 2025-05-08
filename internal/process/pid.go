package process

import (
	"fmt"
	"os"
	"strconv"
)

func WritePIDFile(pidFile string) error {
	pid := os.Getpid()
	if pid == 0 {
		return fmt.Errorf("failed to get process ID")
	}

	pidStr := strconv.Itoa(pid)
	if err := os.WriteFile(pidFile, []byte(pidStr), 0644); err != nil {
		return fmt.Errorf("failed to write PID file: %w", err)
	}

	return nil
}

func RemovePIDFile(pidFile string) {
	_ = os.Remove(pidFile)
}
