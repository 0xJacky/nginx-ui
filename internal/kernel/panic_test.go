package kernel

import (
	"io"
	"os"
	"strings"
	"testing"

	"github.com/gin-gonic/gin"
	"github.com/uozi-tech/cosy/logger"
	cSettings "github.com/uozi-tech/cosy/settings"
)

func TestRecoverWithLocalPanicLogLogsAndRepanicsWithoutSLS(t *testing.T) {
	originalStderr := os.Stderr
	originalSLSSettings := *cSettings.SLSSettings
	originalLogSettings := *cSettings.LogSettings

	reader, writer, err := os.Pipe()
	if err != nil {
		t.Fatalf("create stderr pipe: %v", err)
	}

	*cSettings.SLSSettings = cSettings.SLS{}
	cSettings.LogSettings.EnableFileLog = false
	os.Stderr = writer
	logger.Init(gin.DebugMode)
	defer func() {
		os.Stderr = originalStderr
		logger.Init(gin.DebugMode)
		*cSettings.SLSSettings = originalSLSSettings
		*cSettings.LogSettings = originalLogSettings
		_ = writer.Close()
		_ = reader.Close()
	}()

	recovered := func() (recovered any) {
		defer func() {
			recovered = recover()
		}()
		func() {
			defer RecoverWithLocalPanicLog()
			panic("router registration failed")
		}()
		return nil
	}()

	logger.Sync()
	if err := writer.Close(); err != nil {
		t.Fatalf("close stderr pipe: %v", err)
	}

	output, err := io.ReadAll(reader)
	if err != nil {
		t.Fatalf("read stderr: %v", err)
	}
	if recovered != "router registration failed" {
		t.Fatalf("expected panic to propagate, got %v", recovered)
	}
	logOutput := string(output)
	if !strings.Contains(logOutput, "Application initialization panic before SLS was ready: router registration failed") {
		t.Fatalf("expected local panic message, got %q", logOutput)
	}
	if !strings.Contains(logOutput, "TestRecoverWithLocalPanicLogLogsAndRepanicsWithoutSLS") {
		t.Fatalf("expected stack trace in local panic log, got %q", logOutput)
	}
}
