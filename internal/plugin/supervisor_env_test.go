package plugin

import (
	"slices"
	"strings"
	"testing"
)

func TestSupervisorEnvDropsHostOnlyVariables(t *testing.T) {
	t.Setenv("LEGO_DISABLE_CNAME_SUPPORT", "true")
	t.Setenv("NGINX_UI_NODE_SECRET", "hidden")
	t.Setenv("PLUGIN_ENV_PROBE", "kept")

	s := NewSupervisor(SupervisorConfig{
		PluginID:    "com.example.dns",
		DataDir:     t.TempDir(),
		HostVersion: "2.7.0",
	})

	env := s.env()
	for _, entry := range env {
		for _, hidden := range []string{"LEGO_DISABLE_CNAME_SUPPORT=", "NGINX_UI_NODE_SECRET="} {
			if strings.HasPrefix(entry, hidden) {
				t.Fatalf("the plugin inherited %q", entry)
			}
		}
	}
	if !slices.Contains(env, "PLUGIN_ENV_PROBE=kept") {
		t.Fatal("the plugin lost an unrelated environment variable")
	}
	// The variables the host sets for the plugin itself share the prefix
	// and must still be there.
	if !slices.Contains(env, EnvPluginID+"=com.example.dns") {
		t.Fatal("the plugin id is missing from the environment")
	}
}
