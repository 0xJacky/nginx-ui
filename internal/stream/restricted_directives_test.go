package stream

import (
	"context"
	"errors"
	"os"
	"path/filepath"
	"strings"
	"testing"

	internalconfig "github.com/0xJacky/Nginx-UI/internal/config"
	"github.com/uozi-tech/cosy"
)

// restrictedStreamConfig loads njs into the Nginx worker, which Save refuses
// unless the API handler verified the two-factor step-up first.
const restrictedStreamConfig = "server {\n    listen 8080;\n    js_preread main.preread;\n}\n"

func requireStreamRestrictedDirectiveError(t *testing.T, err error, wantParam string) {
	t.Helper()

	var cosyErr *cosy.Error
	if !errors.As(err, &cosyErr) {
		t.Fatalf("expected cosy error, got %v", err)
	}

	var wantErr *cosy.Error
	if !errors.As(internalconfig.ErrConfigDirectiveNotAllowed, &wantErr) {
		t.Fatal("ErrConfigDirectiveNotAllowed is not a cosy error")
	}

	if cosyErr.Scope != wantErr.Scope || cosyErr.Code != wantErr.Code {
		t.Fatalf("expected cosy error %s:%d, got %s:%d", wantErr.Scope, wantErr.Code, cosyErr.Scope, cosyErr.Code)
	}

	if len(cosyErr.Params) != 1 || cosyErr.Params[0] != wantParam {
		t.Fatalf("expected params [%q], got %v", wantParam, cosyErr.Params)
	}
}

func TestStreamSaveRejectsRestrictedDirectivesByDefault(t *testing.T) {
	confDir, _ := setupStreamMutationTest(t)

	err := Save(context.Background(), "tcp_proxy", restrictedStreamConfig, true, nil, "")
	requireStreamRestrictedDirectiveError(t, err, "js_preread")

	path := filepath.Join(confDir, "streams-available", "tcp_proxy")
	if _, statErr := os.Stat(path); !os.IsNotExist(statErr) {
		t.Fatalf("expected %q to stay absent, stat error: %v", path, statErr)
	}
}

func TestStreamSaveAllowsRestrictedDirectivesWithTheOption(t *testing.T) {
	confDir, _ := setupStreamMutationTest(t)
	err := Save(context.Background(), "tcp_proxy", restrictedStreamConfig, true, nil, "",
		AllowRestrictedDirectives(true))
	if err != nil {
		t.Fatalf("expected the save to be accepted, got %v", err)
	}

	content, readErr := os.ReadFile(filepath.Join(confDir, "streams-available", "tcp_proxy"))
	if readErr != nil {
		t.Fatalf("expected the stream to be written: %v", readErr)
	}
	if !strings.Contains(string(content), "js_preread") {
		t.Fatalf("expected js_preread to be saved, got %q", content)
	}
}

// SaveStreamConfig is the path the API handler takes, and it validates on its
// own before delegating, so the option has to reach both.
func TestSaveStreamConfigHonoursTheOption(t *testing.T) {
	confDir, _ := setupStreamMutationTest(t)
	err := SaveStreamConfig(context.Background(), "tcp_proxy", restrictedStreamConfig, 0, nil, true, "")
	requireStreamRestrictedDirectiveError(t, err, "js_preread")

	err = SaveStreamConfig(context.Background(), "tcp_proxy", restrictedStreamConfig, 0, nil, true, "",
		AllowRestrictedDirectives(true))
	if err != nil {
		t.Fatalf("expected the save to be accepted, got %v", err)
	}

	content, readErr := os.ReadFile(filepath.Join(confDir, "streams-available", "tcp_proxy"))
	if readErr != nil {
		t.Fatalf("expected the stream to be written: %v", readErr)
	}
	if !strings.Contains(string(content), "js_preread") {
		t.Fatalf("expected js_preread to be saved, got %q", content)
	}
}

func TestStreamSaveKeepsRootWorkersBlockedWithTheOption(t *testing.T) {
	confDir, _ := setupStreamMutationTest(t)

	err := Save(context.Background(), "tcp_proxy", "user root;\nserver {\n    listen 8080;\n}\n", true, nil, "",
		AllowRestrictedDirectives(true))
	requireStreamRestrictedDirectiveError(t, err, "user root")

	path := filepath.Join(confDir, "streams-available", "tcp_proxy")
	if _, statErr := os.Stat(path); !os.IsNotExist(statErr) {
		t.Fatalf("expected %q to stay absent, stat error: %v", path, statErr)
	}
}
