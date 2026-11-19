package helper

import (
	"os"
	"path/filepath"
	"testing"
)

func useOfficialImageMarker(t *testing.T, present bool) {
	t.Helper()

	marker := filepath.Join(t.TempDir(), "run")
	if present {
		if err := os.WriteFile(marker, []byte("#!/bin/sh\n"), 0o755); err != nil {
			t.Fatalf("write marker: %v", err)
		}
	}

	original := officialImageMarker
	officialImageMarker = marker
	t.Cleanup(func() { officialImageMarker = original })
}

func TestOfficialDockerImageDetection(t *testing.T) {
	cases := []struct {
		name         string
		env          string
		marker       bool
		ignoreSocket string
		official     bool
		inDocker     bool
		envMissing   bool
	}{
		{name: "env and marker", env: "true", marker: true, official: true, inDocker: true},
		{name: "env only", env: "true", official: true, inDocker: true},
		// A container platform dropped the image ENV; the baked-in marker
		// still identifies the image.
		{name: "marker only", marker: true, official: true, inDocker: true, envMissing: true},
		{name: "neither", official: false},
		// Opting out of the docker socket features keeps the image official.
		{name: "socket opt-out", env: "true", marker: true, ignoreSocket: "true", official: true},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			useOfficialImageMarker(t, tc.marker)
			t.Setenv("NGINX_UI_OFFICIAL_DOCKER", tc.env)
			t.Setenv("NGINX_UI_IGNORE_DOCKER_SOCKET", tc.ignoreSocket)

			if got := IsOfficialDockerImage(); got != tc.official {
				t.Errorf("IsOfficialDockerImage() = %v, want %v", got, tc.official)
			}
			if got := InNginxUIOfficialDocker(); got != tc.inDocker {
				t.Errorf("InNginxUIOfficialDocker() = %v, want %v", got, tc.inDocker)
			}
			if got := OfficialDockerEnvMissing(); got != tc.envMissing {
				t.Errorf("OfficialDockerEnvMissing() = %v, want %v", got, tc.envMissing)
			}
		})
	}
}
