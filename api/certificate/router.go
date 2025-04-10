package certificate

import "github.com/gin-gonic/gin"

func InitDNSCredentialRouter(r *gin.RouterGroup) {
	r.GET("dns_credentials", GetDnsCredentialList)
	r.GET("dns_credentials/:id", GetDnsCredential)
	r.POST("dns_credentials", AddDnsCredential)
	r.POST("dns_credentials/:id", EditDnsCredential)
	r.DELETE("dns_credentials/:id", DeleteDnsCredential)
}

func InitCertificateRouter(r *gin.RouterGroup) {
	r.GET("certs", GetCertList)
	r.GET("certs/:id", GetCert)
	r.POST("certs", AddCert)
	r.POST("certs/:id", ModifyCert)
	r.DELETE("certs/:id", RemoveCert)
	r.PUT("cert_sync", SyncCertificate)
	r.GET("certificate/dns_providers", GetDNSProvidersList)
	r.GET("certificate/dns_provider/:code", GetDNSProvider)
}

func InitCertificateWebSocketRouter(r *gin.RouterGroup) {
	r.GET("domain/:name/cert", IssueCert)
	r.GET("certs/:id/revoke", RevokeCert)
}

func InitAcmeUserRouter(r *gin.RouterGroup) {
	r.GET("acme_users", GetAcmeUserList)
	r.GET("acme_users/:id", GetAcmeUser)
	r.POST("acme_users", CreateAcmeUser)
	r.POST("acme_users/:id", ModifyAcmeUser)
	r.POST("acme_users/:id/register", RegisterAcmeUser)
	r.DELETE("acme_users/:id", DestroyAcmeUser)
	r.PATCH("acme_users/:id", RecoverAcmeUser)
}
