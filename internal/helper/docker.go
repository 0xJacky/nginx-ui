package helper

import (
	"os"

	"github.com/spf13/cast"
)

// IsOfficialDockerImage returns true when running inside the official Nginx UI
// docker image. Unlike InNginxUIOfficialDocker, this does NOT honour
// NGINX_UI_IGNORE_DOCKER_SOCKET — that opt-out is specific to features that
// require the docker socket (OTA upgrade), and should not suppress checks
// that simply happen to be docker-only (e.g. bundled config sync).
func IsOfficialDockerImage() bool {
	return cast.ToBool(os.Getenv("NGINX_UI_OFFICIAL_DOCKER"))
}

// ShouldManageBundledNginx returns true when the official Docker image owns
// the bundled Nginx configuration. Host mode disables that ownership even when
// the process is still running inside the official image.
func ShouldManageBundledNginx() bool {
	return IsOfficialDockerImage() &&
		!cast.ToBool(os.Getenv("NGINX_UI_DISABLE_BUNDLED_NGINX"))
}

func InNginxUIOfficialDocker() bool {
	return cast.ToBool(os.Getenv("NGINX_UI_OFFICIAL_DOCKER")) &&
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
