package helper

import (
	"os"

	"github.com/spf13/cast"
)

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
