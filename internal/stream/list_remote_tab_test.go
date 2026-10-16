package stream

import (
	"context"
	"os"
	"path/filepath"
	"testing"

	"github.com/0xJacky/Nginx-UI/internal/config"
	"github.com/0xJacky/Nginx-UI/model"
	"github.com/0xJacky/Nginx-UI/query"
)

// TestGetStreamConfigsKeepsRemoteNamespaceStreamsOutOfLocalTab guards against a
// stream of a remote-deploy namespace leaking into the local tab with the
// status of the local streams-enabled directory, which it never uses.
func TestGetStreamConfigsKeepsRemoteNamespaceStreamsOutOfLocalTab(t *testing.T) {
	env := setupDeleteTest(t)

	namespace := &model.Namespace{Name: "all-node", DeployMode: model.DeployModeRemote}
	if err := env.db.Create(namespace).Error; err != nil {
		t.Fatalf("failed to create namespace: %v", err)
	}

	for _, name := range []string{"remote-tcp", "local-tcp", "untracked-tcp"} {
		path := filepath.Join(env.availableDir, name)
		if err := os.WriteFile(path, []byte("server {}\n"), 0o644); err != nil {
			t.Fatalf("failed to write %s: %v", name, err)
		}
	}

	records := []*model.Stream{
		{
			Path:          filepath.Join(env.availableDir, "remote-tcp"),
			NamespaceID:   namespace.ID,
			RemoteEnabled: true,
		},
		{Path: filepath.Join(env.availableDir, "local-tcp")},
	}
	if err := env.db.Create(records).Error; err != nil {
		t.Fatalf("failed to create streams: %v", err)
	}

	// Same query as the local tab in api/streams/streams.go, which attaches the
	// namespaces itself.
	s := query.Stream
	streams, err := s.Preload(s.Namespace).Find()
	if err != nil {
		t.Fatalf("failed to load streams: %v", err)
	}

	localConfigs, err := GetStreamConfigs(context.Background(), &ListOptions{}, streams)
	if err != nil {
		t.Fatalf("GetStreamConfigs failed for the local tab: %v", err)
	}

	listed := make(map[string]bool, len(localConfigs))
	for _, cfg := range localConfigs {
		listed[cfg.Name] = true
	}
	if listed["remote-tcp"] {
		t.Fatalf("local tab listed a remote namespace stream: %+v", localConfigs)
	}
	for _, name := range []string{"local-tcp", "untracked-tcp"} {
		if !listed[name] {
			t.Fatalf("local tab is missing %s: %+v", name, localConfigs)
		}
	}

	remoteStreams, err := s.Where(s.NamespaceID.Eq(namespace.ID)).Preload(s.Namespace).Find()
	if err != nil {
		t.Fatalf("failed to load remote namespace streams: %v", err)
	}
	remoteConfigs, err := GetStreamConfigs(context.Background(), &ListOptions{NamespaceID: namespace.ID}, remoteStreams)
	if err != nil {
		t.Fatalf("GetStreamConfigs failed for the namespace tab: %v", err)
	}
	if len(remoteConfigs) != 1 || remoteConfigs[0].Name != "remote-tcp" {
		t.Fatalf("namespace tab = %+v, want only remote-tcp", remoteConfigs)
	}
	if remoteConfigs[0].Status != config.StatusEnabled {
		t.Fatalf("namespace tab status = %q, want %q", remoteConfigs[0].Status, config.StatusEnabled)
	}
}
