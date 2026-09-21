package plugin

import (
	"slices"
	"strings"
	"testing"
)

func TestSupervisorEnvDropsHostOnlyVariables(t *testing.T) {
	t.Setenv("LEGO_DISABLE_CNAME_SUPPORT", "true")
	t.Setenv("NGINX_UI_ENV_PROBE", "kept")

	s := NewSupervisor(SupervisorConfig{
		PluginID:    "com.example.dns",
		DataDir:     t.TempDir(),
		HostVersion: "2.7.0",
	})

	env := s.env()
	for _, entry := range env {
		if strings.HasPrefix(entry, "LEGO_DISABLE_CNAME_SUPPORT=") {
			t.Fatalf("the plugin inherited %q", entry)
		}
	}
	if !slices.Contains(env, "NGINX_UI_ENV_PROBE=kept") {
		t.Fatal("the plugin lost an unrelated environment variable")
	}
	if !slices.Contains(env, EnvPluginID+"=com.example.dns") {
		t.Fatal("the plugin id is missing from the environment")
	}
}
