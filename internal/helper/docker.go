package helper

import (
	"os"

	"github.com/spf13/cast"
)

// officialImageMarker is a file only the official image carries: the s6
// service that runs Nginx UI. The image sets NGINX_UI_OFFICIAL_DOCKER too, but
// a container platform can drop image ENV (an ECS/Fargate task definition or a
// rebuilt image), while files baked into the image always stay. A variable for
// tests.
var officialImageMarker = "/etc/s6-overlay/s6-rc.d/nginx-ui/run"

// IsOfficialDockerImage returns true when running inside the official Nginx UI
// docker image. Unlike InNginxUIOfficialDocker, this does NOT honour
// NGINX_UI_IGNORE_DOCKER_SOCKET — that opt-out is specific to features that
// require the docker socket (OTA upgrade), and should not suppress checks
// that simply happen to be docker-only (e.g. bundled config sync).
func IsOfficialDockerImage() bool {
	return officialDockerEnv() || FileExists(officialImageMarker)
}

// OfficialDockerEnvMissing reports an official image whose container lost the
// NGINX_UI_OFFICIAL_DOCKER variable the image sets. Detection still works
// through the image marker, but the operator should know their platform
// overrides the image environment.
func OfficialDockerEnvMissing() bool {
	return !officialDockerEnv() && FileExists(officialImageMarker)
}

func officialDockerEnv() bool {
	return cast.ToBool(os.Getenv("NGINX_UI_OFFICIAL_DOCKER"))
}

// ShouldManageBundledNginx returns true when the official Docker image owns
// the bundled Nginx configuration. Host mode disables that ownership even when
// the process is still running inside the official image.
func ShouldManageBundledNginx() bool {
	return IsOfficialDockerImage() &&
		!cast.ToBool(os.Getenv("NGINX_UI_DISABLE_BUNDLED_NGINX"))
}

// InNginxUIOfficialDocker returns true inside the official image unless the
// operator opted out of the docker socket features (OTA upgrade).
func InNginxUIOfficialDocker() bool {
	return IsOfficialDockerImage() &&
		!cast.ToBool(os.Getenv("NGINX_UI_IGNORE_DOCKER_SOCKET"))
}

func DockerSocketExists() bool {
	if !InNginxUIOfficialDocker() {
		return false
	}
	_, err := os.Stat("/var/run/docker.sock")
	if os.IsNotExist(err) {
		return false
	}
	return true
}
