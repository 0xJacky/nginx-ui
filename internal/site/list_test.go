package site

import (
	"testing"

	"github.com/0xJacky/Nginx-UI/internal/config"
	"github.com/0xJacky/Nginx-UI/model"
)

func TestApplySiteDescriptions(t *testing.T) {
	configs := []config.Config{{Name: "example.com"}, {Name: "other.conf"}}
	sites := []*model.Site{
		{Path: "/etc/nginx/sites-available/example.com", Description: "Customer portal"},
		{Path: "/etc/nginx/sites-available/unlisted.conf", Description: "Unused"},
	}

	result := applySiteDescriptions(configs, sites)
	if result[0].Description != "Customer portal" {
		t.Fatalf("description = %q, want Customer portal", result[0].Description)
	}
	if result[1].Description != "" {
		t.Fatalf("unmatched description = %q, want empty", result[1].Description)
	}
}
