package upgrader

import (
	"errors"
	"sync/atomic"
	"testing"
	"time"

	"github.com/stretchr/testify/require"
	"github.com/uozi-tech/cosy/logger"
)

type progressWriterFunc func(any) error

func (write progressWriterFunc) WriteJSON(value any) error {
	return write(value)
}

func TestUpgradeProgressDrainsAfterClientDisconnects(t *testing.T) {
	logger.Init("debug")
	var writes atomic.Int32
	progress, stop := startUpgradeProgress(progressWriterFunc(func(any) error {
		writes.Add(1)
		return errors.New("client disconnected")
	}), "Pulling Docker image...")
	defer stop()

	// Model an unbuffered Docker producer: every progress send must complete
	// even after the browser disappears, or the upgrade can never finish.
	for _, value := range []float64{10, 50, 100} {
		select {
		case progress <- value:
		case <-time.After(time.Second):
			t.Fatal("upgrade producer blocked after its progress connection failed")
		}
	}
	require.EqualValues(t, 1, writes.Load())
}

func TestUpgradeProgressPreservesFramesAndFinishesWritesBeforeReturning(t *testing.T) {
	var frames []CoreUpgradeResp
	progress, stop := startUpgradeProgress(progressWriterFunc(func(value any) error {
		frame, ok := value.(CoreUpgradeResp)
		if !ok {
			return errors.New("unexpected progress frame type")
		}
		frames = append(frames, frame)
		return nil
	}), "Pulling Docker image...")
	for _, value := range []float64{0, 50, 100} {
		progress <- value
	}
	stop()
	require.Equal(t, []CoreUpgradeResp{
		{Status: UpgradeStatusProgress, Progress: 0, Message: "Pulling Docker image..."},
		{Status: UpgradeStatusProgress, Progress: 50, Message: "Pulling Docker image..."},
		{Status: UpgradeStatusProgress, Progress: 100, Message: "Pulling Docker image..."},
	}, frames)
}

func TestUpgradeProgressHandlesNoFrames(t *testing.T) {
	var writes atomic.Int32
	_, stop := startUpgradeProgress(progressWriterFunc(func(any) error {
		writes.Add(1)
		return nil
	}), "")
	stop()
	require.Zero(t, writes.Load())
}
