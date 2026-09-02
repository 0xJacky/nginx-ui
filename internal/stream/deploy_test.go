package stream

import (
	"testing"

	"github.com/0xJacky/Nginx-UI/internal/config"
	"github.com/0xJacky/Nginx-UI/model"
)

func TestRemoteStatusMapsDeploymentIntent(t *testing.T) {
	if got := remoteStatus(true); got != config.StatusEnabled {
		t.Fatalf("got %q, want %q", got, config.StatusEnabled)
	}
	if got := remoteStatus(false); got != config.StatusDisabled {
		t.Fatalf("got %q, want %q", got, config.StatusDisabled)
	}
}

func TestApplyRemoteStatusOverridesFilesystemStatus(t *testing.T) {
	configs := []config.Config{
		{Name: "remote.conf", Status: config.StatusDisabled},
		{Name: "local.conf", Status: config.StatusEnabled},
	}
	streams := []*model.Stream{
		{
			Path:          "/etc/nginx/streams-available/remote.conf",
			RemoteEnabled: true,
			Namespace:     &model.Namespace{DeployMode: model.DeployModeRemote},
		},
		{
			Path:      "/etc/nginx/streams-available/local.conf",
			Namespace: &model.Namespace{DeployMode: model.DeployModeLocal},
		},
	}

	got := applyRemoteStatus(configs, streams, "")

	if len(got) != 2 {
		t.Fatalf("expected both streams, got %d", len(got))
	}
	if got[0].Status != config.StatusEnabled {
		t.Fatalf("remote stream must report its deployment intent, got %q", got[0].Status)
	}
	if got[1].Status != config.StatusEnabled {
		t.Fatalf("local stream status must be untouched, got %q", got[1].Status)
	}
}

func TestApplyRemoteStatusReappliesStatusFilter(t *testing.T) {
	configs := []config.Config{
		{Name: "remote.conf", Status: config.StatusDisabled},
		{Name: "other.conf", Status: config.StatusEnabled},
	}
	streams := []*model.Stream{
		{
			Path:      "/etc/nginx/streams-available/remote.conf",
			Namespace: &model.Namespace{DeployMode: model.DeployModeRemote},
		},
	}

	got := applyRemoteStatus(configs, streams, string(config.StatusEnabled))

	if len(got) != 1 || got[0].Name != "other.conf" {
		t.Fatalf("the disabled remote stream must be filtered out, got %+v", got)
	}
}
