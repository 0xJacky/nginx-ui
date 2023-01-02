package api

import (
	"github.com/0xJacky/Nginx-UI/server/model"
	"github.com/0xJacky/Nginx-UI/server/pkg/cert"
	"github.com/0xJacky/Nginx-UI/server/pkg/nginx"
	"github.com/gin-gonic/gin"
	"github.com/gorilla/websocket"
	"github.com/spf13/cast"
	"log"
	"net/http"
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
			log.Println("api.handleIssueCertLogChan recover", err)
		}
	}()

	for logString := range logChan {

		err := conn.WriteJSON(IssueCertResponse{
			Status:  Info,
			Message: logString,
		})

		if err != nil {
			log.Println("Error handleIssueCertLogChan", err)
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
		log.Println(err)
		return
	}

	defer func(ws *websocket.Conn) {
		err := ws.Close()
		if err != nil {
			log.Println("defer websocket close err", err)
		}
	}(ws)

	// read
	var buffer struct {
		ServerName []string `json:"server_name"`
	}

	err = ws.ReadJSON(&buffer)

	if err != nil {
		log.Println(err)
		return
	}

	logChan := make(chan string, 1)
	errChan := make(chan error, 1)

	go cert.IssueCert(buffer.ServerName, logChan, errChan)

	domain := strings.Join(buffer.ServerName, "_")

	go handleIssueCertLogChan(ws, logChan)

	// block, unless errChan closed
	for err = range errChan {
		log.Println("Error cert.IssueCert", err)

		err = ws.WriteJSON(IssueCertResponse{
			Status:  Error,
			Message: err.Error(),
		})

		if err != nil {
			log.Println("Error WriteJSON", err)
			return
		}

		return
	}

	close(logChan)

	sslCertificatePath := nginx.GetNginxConfPath("ssl/" + domain + "/fullchain.cer")
	sslCertificateKeyPath := nginx.GetNginxConfPath("ssl/" + domain + "/private.key")

	certModel, err := model.FirstOrCreateCert(domain)

	if err != nil {
		log.Println(err)
	}

	err = certModel.Updates(&model.Cert{
		SSLCertificatePath:    sslCertificatePath,
		SSLCertificateKeyPath: sslCertificateKeyPath,
	})

	if err != nil {
		log.Println(err)
	}

	err = ws.WriteJSON(IssueCertResponse{
		Status:            Success,
		Message:           "Issued certificate successfully",
		SSLCertificate:    sslCertificatePath,
		SSLCertificateKey: sslCertificateKeyPath,
	})

	if err != nil {
		log.Println(err)
		return
	}

}

func GetCertList(c *gin.Context) {
	certList := model.GetCertList(c.Query("name"), c.Query("domain"))

	c.JSON(http.StatusOK, gin.H{
		"data": certList,
	})
}

func GetCert(c *gin.Context) {
	certModel, err := model.FirstCertByID(cast.ToInt(c.Param("id")))

	if err != nil {
		ErrHandler(c, err)
		return
	}

	c.JSON(http.StatusOK, certModel)
}

func AddCert(c *gin.Context) {
	var json struct {
		Name                  string `json:"name" binding:"required"`
		Domain                string `json:"domain" binding:"required"`
		SSLCertificatePath    string `json:"ssl_certificate_path" binding:"required"`
		SSLCertificateKeyPath string `json:"ssl_certificate_key_path" binding:"required"`
	}
	if !BindAndValid(c, &json) {
		return
	}
	certModel, err := model.FirstOrCreateCert(json.Domain)

	if err != nil {
		ErrHandler(c, err)
		return
	}

	err = certModel.Updates(&model.Cert{
		Name:                  json.Name,
		Domain:                json.Domain,
		SSLCertificatePath:    json.SSLCertificatePath,
		SSLCertificateKeyPath: json.SSLCertificateKeyPath,
	})

	if err != nil {
		ErrHandler(c, err)
		return
	}

	c.JSON(http.StatusOK, nil)
}

func ModifyCert(c *gin.Context) {
	id := cast.ToInt(c.Param("id"))
	certModel, err := model.FirstCertByID(id)

	var json struct {
		Name                  string `json:"name" binding:"required"`
		Domain                string `json:"domain" binding:"required"`
		SSLCertificatePath    string `json:"ssl_certificate_path" binding:"required"`
		SSLCertificateKeyPath string `json:"ssl_certificate_key_path" binding:"required"`
	}

	if !BindAndValid(c, &json) {
		return
	}

	if err != nil {
		ErrHandler(c, err)
		return
	}

	err = certModel.Updates(&model.Cert{
		Name:                  json.Name,
		Domain:                json.Domain,
		SSLCertificatePath:    json.SSLCertificatePath,
		SSLCertificateKeyPath: json.SSLCertificateKeyPath,
	})

	if err != nil {
		ErrHandler(c, err)
		return
	}

	c.JSON(http.StatusOK, certModel)
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

	c.JSON(http.StatusOK, nil)
}
