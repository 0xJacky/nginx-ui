package api

import (
	"github.com/0xJacky/Nginx-UI/internal/cert"
	"github.com/0xJacky/Nginx-UI/internal/cert/dns"
	"github.com/0xJacky/Nginx-UI/internal/logger"
	"github.com/0xJacky/Nginx-UI/internal/nginx"
	"github.com/0xJacky/Nginx-UI/model"
	"github.com/gin-gonic/gin"
	"github.com/gorilla/websocket"
	"github.com/spf13/cast"
	"net/http"
	"os"
	"path/filepath"
	"strings"
)

const (
	Success = "success"
	Info    = "info"
	Error   = "error"
)

type IssueCertResponse struct {
	Status            string `json:"status"`
	Message           string `json:"message"`
	SSLCertificate    string `json:"ssl_certificate,omitempty"`
	SSLCertificateKey string `json:"ssl_certificate_key,omitempty"`
}

func handleIssueCertLogChan(conn *websocket.Conn, logChan chan string) {
	defer func() {
		if err := recover(); err != nil {
			logger.Error(err)
		}
	}()

	for logString := range logChan {

		err := conn.WriteJSON(IssueCertResponse{
			Status:  Info,
			Message: logString,
		})

		if err != nil {
			logger.Error(err)
			return
		}

	}
}

func IssueCert(c *gin.Context) {
	var upGrader = websocket.Upgrader{
		CheckOrigin: func(r *http.Request) bool {
			return true
		},
	}

	// upgrade http to websocket
	ws, err := upGrader.Upgrade(c.Writer, c.Request, nil)
	if err != nil {
		logger.Error(err)
		return
	}

	defer func(ws *websocket.Conn) {
		_ = ws.Close()
	}(ws)

	// read
	buffer := &cert.ConfigPayload{}

	err = ws.ReadJSON(buffer)

	if err != nil {
		logger.Error(err)
		return
	}

	certModel, err := model.FirstOrCreateCert(c.Param("name"))

	if err != nil {
		logger.Error(err)
		return
	}

	logChan := make(chan string, 1)
	errChan := make(chan error, 1)

	go cert.IssueCert(buffer, logChan, errChan)

	go handleIssueCertLogChan(ws, logChan)

	// block, until errChan closes
	for err = range errChan {
		errLog := &cert.AutoCertErrorLog{}
		errLog.SetCertModel(&certModel)
		errLog.Exit("issue cert", err)

		err = ws.WriteJSON(IssueCertResponse{
			Status:  Error,
			Message: err.Error(),
		})

		if err != nil {
			logger.Error(err)
			return
		}

		return
	}

	certDirName := strings.Join(buffer.ServerName, "_")
	sslCertificatePath := nginx.GetConfPath("ssl", certDirName, "fullchain.cer")
	sslCertificateKeyPath := nginx.GetConfPath("ssl", certDirName, "private.key")

	err = certModel.Updates(&model.Cert{
		Domains:               buffer.ServerName,
		SSLCertificatePath:    sslCertificatePath,
		SSLCertificateKeyPath: sslCertificateKeyPath,
	})

	if err != nil {
		logger.Error(err)
		err = ws.WriteJSON(IssueCertResponse{
			Status:  Error,
			Message: err.Error(),
		})
		return
	}

	certModel.ClearLog()

	err = ws.WriteJSON(IssueCertResponse{
		Status:            Success,
		Message:           "Issued certificate successfully",
		SSLCertificate:    sslCertificatePath,
		SSLCertificateKey: sslCertificateKeyPath,
	})

	if err != nil {
		logger.Error(err)
		return
	}

}

func GetCertList(c *gin.Context) {
	certList := model.GetCertList(c.Query("name"), c.Query("domain"))

	c.JSON(http.StatusOK, gin.H{
		"data": certList,
	})
}

func getCert(c *gin.Context, certModel *model.Cert) {
	type resp struct {
		*model.Cert
		SSLCertification    string           `json:"ssl_certification"`
		SSLCertificationKey string           `json:"ssl_certification_key"`
		CertificateInfo     *CertificateInfo `json:"certificate_info,omitempty"`
	}

	var sslCertificationBytes, sslCertificationKeyBytes []byte
	var certificateInfo *CertificateInfo
	if certModel.SSLCertificatePath != "" {
		if _, err := os.Stat(certModel.SSLCertificatePath); err == nil {
			sslCertificationBytes, _ = os.ReadFile(certModel.SSLCertificatePath)
		}

		pubKey, err := cert.GetCertInfo(certModel.SSLCertificatePath)

		if err != nil {
			ErrHandler(c, err)
			return
		}

		certificateInfo = &CertificateInfo{
			SubjectName: pubKey.Subject.CommonName,
			IssuerName:  pubKey.Issuer.CommonName,
			NotAfter:    pubKey.NotAfter,
			NotBefore:   pubKey.NotBefore,
		}
	}

	if certModel.SSLCertificateKeyPath != "" {
		if _, err := os.Stat(certModel.SSLCertificateKeyPath); err == nil {
			sslCertificationKeyBytes, _ = os.ReadFile(certModel.SSLCertificateKeyPath)
		}
	}

	c.JSON(http.StatusOK, resp{
		certModel,
		string(sslCertificationBytes),
		string(sslCertificationKeyBytes),
		certificateInfo,
	})
}

func GetCert(c *gin.Context) {
	certModel, err := model.FirstCertByID(cast.ToInt(c.Param("id")))

	if err != nil {
		ErrHandler(c, err)
		return
	}

	getCert(c, &certModel)
}

func AddCert(c *gin.Context) {
	var json struct {
		Name                  string `json:"name"`
		SSLCertificatePath    string `json:"ssl_certificate_path" binding:"required"`
		SSLCertificateKeyPath string `json:"ssl_certificate_key_path" binding:"required"`
		SSLCertification      string `json:"ssl_certification"`
		SSLCertificationKey   string `json:"ssl_certification_key"`
	}
	if !BindAndValid(c, &json) {
		return
	}
	certModel := &model.Cert{
		Name:                  json.Name,
		SSLCertificatePath:    json.SSLCertificatePath,
		SSLCertificateKeyPath: json.SSLCertificateKeyPath,
	}

	err := certModel.Insert()

	if err != nil {
		ErrHandler(c, err)
		return
	}

	err = os.MkdirAll(filepath.Dir(json.SSLCertificatePath), 0644)
	if err != nil {
		ErrHandler(c, err)
		return
	}

	err = os.MkdirAll(filepath.Dir(json.SSLCertificateKeyPath), 0644)
	if err != nil {
		ErrHandler(c, err)
		return
	}

	if json.SSLCertification != "" {
		err = os.WriteFile(json.SSLCertificatePath, []byte(json.SSLCertification), 0644)
		if err != nil {
			ErrHandler(c, err)
			return
		}
	}

	if json.SSLCertificationKey != "" {
		err = os.WriteFile(json.SSLCertificateKeyPath, []byte(json.SSLCertificationKey), 0644)
		if err != nil {
			ErrHandler(c, err)
			return
		}
	}

	getCert(c, certModel)
}

func ModifyCert(c *gin.Context) {
	id := cast.ToInt(c.Param("id"))

	var json struct {
		Name                  string `json:"name"`
		SSLCertificatePath    string `json:"ssl_certificate_path" binding:"required"`
		SSLCertificateKeyPath string `json:"ssl_certificate_key_path" binding:"required"`
		SSLCertification      string `json:"ssl_certification"`
		SSLCertificationKey   string `json:"ssl_certification_key"`
	}

	if !BindAndValid(c, &json) {
		return
	}

	certModel, err := model.FirstCertByID(id)
	if err != nil {
		ErrHandler(c, err)
		return
	}

	err = certModel.Updates(&model.Cert{
		Name:                  json.Name,
		SSLCertificatePath:    json.SSLCertificatePath,
		SSLCertificateKeyPath: json.SSLCertificateKeyPath,
	})

	if err != nil {
		ErrHandler(c, err)
		return
	}

	err = os.MkdirAll(filepath.Dir(json.SSLCertificatePath), 0644)
	if err != nil {
		ErrHandler(c, err)
		return
	}

	err = os.MkdirAll(filepath.Dir(json.SSLCertificateKeyPath), 0644)
	if err != nil {
		ErrHandler(c, err)
		return
	}

	if json.SSLCertification != "" {
		err = os.WriteFile(json.SSLCertificatePath, []byte(json.SSLCertification), 0644)
		if err != nil {
			ErrHandler(c, err)
			return
		}
	}

	if json.SSLCertificationKey != "" {
		err = os.WriteFile(json.SSLCertificateKeyPath, []byte(json.SSLCertificationKey), 0644)
		if err != nil {
			ErrHandler(c, err)
			return
		}
	}

	GetCert(c)
}

func RemoveCert(c *gin.Context) {
	id := cast.ToInt(c.Param("id"))
	certModel, err := model.FirstCertByID(id)

	if err != nil {
		ErrHandler(c, err)
		return
	}

	err = certModel.Remove()

	if err != nil {
		ErrHandler(c, err)
		return
	}

	c.JSON(http.StatusNoContent, nil)
}

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
