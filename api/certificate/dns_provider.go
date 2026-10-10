package certificate

import (
	"net/http"

	"github.com/0xJacky/Nginx-UI/internal/cert"
	"github.com/0xJacky/Nginx-UI/internal/cert/dns"
	"github.com/gin-gonic/gin"
)

func GetDNSProvidersList(c *gin.Context) {
	c.JSON(http.StatusOK, dns.GetProvidersList())
}

func GetDNSProvider(c *gin.Context) {
	code := c.Param("code")

	provider, ok := dns.GetProvider(code)

	if !ok {
		c.JSON(http.StatusNotFound, gin.H{
			"message": "provider not found",
		})
		return
	}

	c.JSON(http.StatusOK, provider)
}

// ChallengeMethod is one way to prove control over an identifier. HTTP-01 is
// part of the core, DNS-01 only exists while a plugin provides it.
type ChallengeMethod struct {
	Code     string `json:"code"`
	Builtin  bool   `json:"builtin,omitempty"`
	PluginID string `json:"plugin_id,omitempty"`
}

// GetChallengeMethods lists what the certificate form may offer right now.
func GetChallengeMethods(c *gin.Context) {
	c.JSON(http.StatusOK, challengeMethods())
}

func challengeMethods() []ChallengeMethod {
	methods := []ChallengeMethod{{Code: cert.HTTP01, Builtin: true}}
	if dns.HasDNS01Providers() {
		methods = append(methods, ChallengeMethod{Code: cert.DNS01, PluginID: dns01PluginID()})
	}
	return methods
}

// dns01PluginID names the plugin the UI should point at. The official plugin
// wins when it is installed, otherwise the one covering the most vendors.
func dns01PluginID() string {
	counts := make(map[string]int)
	for _, provider := range dns.GetProvidersList() {
		if !provider.DNS01 || provider.PluginID == "" {
			continue
		}
		if provider.PluginID == dns.OfficialDNS01PluginID {
			return dns.OfficialDNS01PluginID
		}
		counts[provider.PluginID]++
	}

	best := ""
	for pluginID, count := range counts {
		if count > counts[best] || (count == counts[best] && (best == "" || pluginID < best)) {
			best = pluginID
		}
	}
	return best
}
