package certificate

import "github.com/gin-gonic/gin"

func InitDNSCredentialRouter(r *gin.RouterGroup) {
	r.GET("dns_credentials", GetDnsCredentialList)
	r.GET("dns_credential/:id", GetDnsCredential)
	r.POST("dns_credential", AddDnsCredential)
	r.POST("dns_credential/:id", EditDnsCredential)
	r.DELETE("dns_credential/:id", DeleteDnsCredential)
}

func InitCertificateRouter(r *gin.RouterGroup) {
	r.GET("domain/:name/cert", IssueCert)
	r.GET("certs", GetCertList)
	r.GET("cert/:id", GetCert)
	r.POST("cert", AddCert)
	r.POST("cert/:id", ModifyCert)
	r.DELETE("cert/:id", RemoveCert)
	r.GET("auto_cert/dns/providers", GetDNSProvidersList)
	r.GET("auto_cert/dns/provider/:code", GetDNSProvider)
}
