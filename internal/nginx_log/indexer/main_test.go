package indexer

import (
	"os"
	"testing"
)

func TestMain(m *testing.M) {
	InitLogParser()
	code := m.Run()
	os.Exit(code)
}
