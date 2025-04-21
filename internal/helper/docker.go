package helper

import (
	"os"

	"github.com/spf13/cast"
)

func InNginxUIOfficialDocker() bool {
	return cast.ToBool(os.Getenv("NGINX_UI_OFFICIAL_DOCKER"))
}
