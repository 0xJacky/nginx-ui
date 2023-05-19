package server

import (
	"github.com/0xJacky/Nginx-UI/server/internal/boot"
	"github.com/0xJacky/Nginx-UI/server/internal/logger"
	"github.com/0xJacky/Nginx-UI/server/internal/nginx"
	"github.com/0xJacky/Nginx-UI/server/internal/upgrader"
	"github.com/0xJacky/Nginx-UI/server/router"
	"github.com/jpillora/overseer"
	"net/http"
)

func GetRuntimeInfo() (r upgrader.RuntimeInfo, err error) {
	return upgrader.GetRuntimeInfo()
}

func Program(state overseer.State) {
	defer logger.Sync()

	logger.Info("Nginx config dir path: " + nginx.GetConfPath())

	boot.Kernel()

	err := http.Serve(state.Listener, router.InitRouter())
	if err != nil {
		logger.Error(err)
	}

	logger.Info("Server exiting")
}
