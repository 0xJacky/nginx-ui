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
	r.GET("certs", GetCertList)
	r.GET("cert/:id", GetCert)
	r.POST("cert", AddCert)
	r.POST("cert/:id", ModifyCert)
	r.DELETE("cert/:id", RemoveCert)
	r.PUT("cert_sync", SyncCertificate)
	r.GET("certificate/dns_providers", GetDNSProvidersList)
	r.GET("certificate/dns_provider/:code", GetDNSProvider)
}

func InitCertificateWebSocketRouter(r *gin.RouterGroup) {
	r.GET("domain/:name/cert", IssueCert)
}

func InitAcmeUserRouter(r *gin.RouterGroup) {
	r.GET("acme_user", GetAcmeUserList)
	r.GET("acme_user/:id", GetAcmeUser)
	r.POST("acme_user", CreateAcmeUser)
	r.POST("acme_user/:id", ModifyAcmeUser)
	r.POST("acme_user/:id/register", RegisterAcmeUser)
	r.DELETE("acme_user/:id", DestroyAcmeUser)
	r.PATCH("acme_user/:id", RecoverAcmeUser)
}
