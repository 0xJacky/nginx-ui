package self_check

import (
	"github.com/0xJacky/Nginx-UI/internal/helper"
)

func CheckDockerSocket() error {
	if !helper.InNginxUIOfficialDocker() {
		return nil
	}

	if !helper.DockerSocketExists() {
		return ErrDockerSocketNotExist
	}

	return nil
}
