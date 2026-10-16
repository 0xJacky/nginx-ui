package site

import (
	"context"
	"os"
	"path/filepath"
	"testing"

	"github.com/0xJacky/Nginx-UI/internal/config"
	"github.com/0xJacky/Nginx-UI/model"
	"github.com/0xJacky/Nginx-UI/query"
)

// TestGetSiteConfigsKeepsRemoteNamespaceSitesOutOfLocalTab guards against a
// site of a remote-deploy namespace leaking into the local tab. It used to be
// listed there with no namespace and the status of the local sites-enabled
// directory, so enabling it succeeded on the member nodes but the row flipped
// back to disabled on the next refresh.
func TestGetSiteConfigsKeepsRemoteNamespaceSitesOutOfLocalTab(t *testing.T) {
	database, _, confDir := setupSiteStatusTest(t)
	model.Use(database)

	namespace := &model.Namespace{Name: "all-node", DeployMode: model.DeployModeRemote}
	if err := database.Create(namespace).Error; err != nil {
		t.Fatalf("failed to create namespace: %v", err)
	}

	for _, name := range []string{"remote.example.com", "local.example.com", "untracked.example.com"} {
		path := filepath.Join(confDir, "sites-available", name)
		if err := os.WriteFile(path, []byte("server {}\n"), 0o644); err != nil {
			t.Fatalf("failed to write %s: %v", name, err)
		}
	}

	records := []*model.Site{
		{
			Path:          filepath.Join(confDir, "sites-available", "remote.example.com"),
			NamespaceID:   namespace.ID,
			RemoteEnabled: true,
		},
		{Path: filepath.Join(confDir, "sites-available", "local.example.com")},
	}
	if err := database.Create(records).Error; err != nil {
		t.Fatalf("failed to create sites: %v", err)
	}

	// Same query as the local tab in api/sites/list.go.
	s := query.Site
	sites, err := s.Preload(s.Namespace).Find()
	if err != nil {
		t.Fatalf("failed to load sites: %v", err)
	}

	localConfigs, err := GetSiteConfigs(context.Background(), &ListOptions{}, sites)
	if err != nil {
		t.Fatalf("GetSiteConfigs failed for the local tab: %v", err)
	}

	listed := make(map[string]bool, len(localConfigs))
	for _, cfg := range localConfigs {
		listed[cfg.Name] = true
	}
	if listed["remote.example.com"] {
		t.Fatalf("local tab listed a remote namespace site: %+v", localConfigs)
	}
	for _, name := range []string{"local.example.com", "untracked.example.com"} {
		if !listed[name] {
			t.Fatalf("local tab is missing %s: %+v", name, localConfigs)
		}
	}

	remoteSites, err := s.Where(s.NamespaceID.Eq(namespace.ID)).Preload(s.Namespace).Find()
	if err != nil {
		t.Fatalf("failed to load remote namespace sites: %v", err)
	}
	remoteConfigs, err := GetSiteConfigs(context.Background(), &ListOptions{NamespaceID: namespace.ID}, remoteSites)
	if err != nil {
		t.Fatalf("GetSiteConfigs failed for the namespace tab: %v", err)
	}
	if len(remoteConfigs) != 1 || remoteConfigs[0].Name != "remote.example.com" {
		t.Fatalf("namespace tab = %+v, want only remote.example.com", remoteConfigs)
	}
	if remoteConfigs[0].Status != config.StatusEnabled {
		t.Fatalf("namespace tab status = %q, want %q", remoteConfigs[0].Status, config.StatusEnabled)
	}
}
