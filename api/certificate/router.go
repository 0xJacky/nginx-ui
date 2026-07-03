package certificate

import (
	"github.com/0xJacky/Nginx-UI/internal/middleware"
	"github.com/gin-gonic/gin"
)

func InitDNSCredentialRouter(r *gin.RouterGroup) {
	r.GET("dns_credentials", GetDnsCredentialList)
	r.GET("dns_credentials/:id", GetDnsCredential)
	o := r.Group("", middleware.RequireSecureSession())
	{
		o.POST("dns_credentials", AddDnsCredential)
		o.POST("dns_credentials/:id", EditDnsCredential)
		o.DELETE("dns_credentials/:id", DeleteDnsCredential)
	}
}

func InitCertificateRouter(r *gin.RouterGroup) {
	r.GET("certs", GetCertList)
	r.GET("certs/:id", GetCert)
	r.GET("certificate/dns_providers", GetDNSProvidersList)
	r.GET("certificate/dns_provider/:code", GetDNSProvider)
	o := r.Group("", middleware.RequireSecureSession())
	{
		o.POST("certs", AddCert)
		o.POST("certs/:id", ModifyCert)
		o.DELETE("certs/:id", RemoveCert)
		o.POST("cert_import", ImportExistingCert)
		o.POST("cert_discover_new", DiscoverNewCerts)
		o.PUT("cert_sync", SyncCertificate)
		o.POST("self_signed_cert", GenerateSelfSignedCert)
		o.POST("self_signed_cert/:id", ModifySelfSignedCert)
	}
}

func InitCertificateWebSocketRouter(r *gin.RouterGroup) {
	o := r.Group("", middleware.RequireSecureSession())
	{
		o.GET("domain/:name/cert", IssueCert)
		o.GET("certs/:id/revoke", RevokeCert)
	}
}

func InitAcmeUserRouter(r *gin.RouterGroup) {
	r.GET("acme_users", GetAcmeUserList)
	r.GET("acme_users/:id", GetAcmeUser)
	o := r.Group("", middleware.RequireSecureSession())
	{
		o.POST("acme_users", CreateAcmeUser)
		o.POST("acme_users/:id", ModifyAcmeUser)
		o.POST("acme_users/:id/register", RegisterAcmeUser)
		o.DELETE("acme_users/:id", DestroyAcmeUser)
		o.PATCH("acme_users/:id", RecoverAcmeUser)
	}
}
