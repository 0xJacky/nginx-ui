package router

import (
	"slices"

	"github.com/0xJacky/Nginx-UI/internal/helper"
	"github.com/0xJacky/Nginx-UI/settings"
	"github.com/gin-gonic/gin"
)

var bundledNginxTrustedProxies = []string{"127.0.0.1", "::1"}

func trustedProxiesForCurrentTopology() []string {
	trustedProxies := slices.Clone(settings.AuthSettings.TrustedProxies)
	if !helper.ShouldManageBundledNginx() {
		return trustedProxies
	}

	for _, trustedProxy := range bundledNginxTrustedProxies {
		if !slices.Contains(trustedProxies, trustedProxy) {
			trustedProxies = append(trustedProxies, trustedProxy)
		}
	}

	return trustedProxies
}

func configureTrustedProxies(engine *gin.Engine) error {
	return engine.SetTrustedProxies(trustedProxiesForCurrentTopology())
}
