package site

import (
	"testing"

	"github.com/0xJacky/Nginx-UI/internal/config"
	"github.com/0xJacky/Nginx-UI/model"
)

func TestRemoteStatusMapsDeploymentIntent(t *testing.T) {
	if got := remoteStatus(true); got != StatusEnabled {
		t.Fatalf("got %q, want %q", got, StatusEnabled)
	}
	if got := remoteStatus(false); got != StatusDisabled {
		t.Fatalf("got %q, want %q", got, StatusDisabled)
	}
}

func TestApplyRemoteStatusOverridesFilesystemStatus(t *testing.T) {
	// A remote site has no local symlink, so the filesystem reports "disabled"
	// even though it is deployed on the member nodes.
	configs := []config.Config{
		{Name: "remote.conf", Status: config.StatusDisabled},
		{Name: "local.conf", Status: config.StatusEnabled},
	}
	sites := []*model.Site{
		{
			Path:          "/etc/nginx/sites-available/remote.conf",
			RemoteEnabled: true,
			Namespace:     &model.Namespace{DeployMode: model.DeployModeRemote},
		},
		{
			Path:      "/etc/nginx/sites-available/local.conf",
			Namespace: &model.Namespace{DeployMode: model.DeployModeLocal},
		},
	}

	got := applyRemoteStatus(configs, sites, "")

	if len(got) != 2 {
		t.Fatalf("expected both sites, got %d", len(got))
	}
	if got[0].Status != config.StatusEnabled {
		t.Fatalf("remote site must report its deployment intent, got %q", got[0].Status)
	}
	if got[1].Status != config.StatusEnabled {
		t.Fatalf("local site status must be untouched, got %q", got[1].Status)
	}
}

func TestApplyRemoteStatusReappliesStatusFilter(t *testing.T) {
	configs := []config.Config{
		{Name: "remote.conf", Status: config.StatusDisabled},
		{Name: "other.conf", Status: config.StatusEnabled},
	}
	sites := []*model.Site{
		{
			Path:          "/etc/nginx/sites-available/remote.conf",
			RemoteEnabled: false,
			Namespace:     &model.Namespace{DeployMode: model.DeployModeRemote},
		},
	}

	got := applyRemoteStatus(configs, sites, string(config.StatusEnabled))

	if len(got) != 1 || got[0].Name != "other.conf" {
		t.Fatalf("the disabled remote site must be filtered out, got %+v", got)
	}
}

func TestApplyRemoteStatusIsANoOpWithoutRemoteSites(t *testing.T) {
	configs := []config.Config{{Name: "local.conf", Status: config.StatusEnabled}}
	sites := []*model.Site{{Path: "/etc/nginx/sites-available/local.conf"}}

	got := applyRemoteStatus(configs, sites, "")

	if len(got) != 1 || got[0].Status != config.StatusEnabled {
		t.Fatalf("unexpected result %+v", got)
	}
}
