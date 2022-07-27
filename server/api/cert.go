package api

import (
	"encoding/json"
	"github.com/0xJacky/Nginx-UI/server/settings"
	"github.com/0xJacky/Nginx-UI/server/tool"
	"github.com/0xJacky/Nginx-UI/server/tool/nginx"
	"github.com/gin-gonic/gin"
	"github.com/gorilla/websocket"
	"log"
	"net/http"
	"os"
)

func CertInfo(c *gin.Context) {
	domain := c.Param("domain")

	key, err := tool.GetCertInfo(domain)

	c.JSON(http.StatusOK, gin.H{
		"error":        err,
		"subject_name": key.Subject.CommonName,
		"issuer_name":  key.Issuer.CommonName,
		"not_after":    key.NotAfter,
		"not_before":   key.NotBefore,
	})
}

func IssueCert(c *gin.Context) {
	domain := c.Param("domain")
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

	for {
		// read
		mt, message, err := ws.ReadMessage()
		if err != nil {
			break
		}
		if string(message) == "go" {
			var m []byte

			if settings.ServerSettings.Demo {
				m, _ = json.Marshal(gin.H{
					"status":  "error",
					"message": "this feature is not available in demo",
				})
				_ = ws.WriteMessage(mt, m)
				return
			}

			err = tool.IssueCert(domain)

			if err != nil {

				log.Println(err)

				m, err = json.Marshal(gin.H{
					"status":  "error",
					"message": err.Error(),
				})

				if err != nil {
					log.Println(err)
					return
				}

				err = ws.WriteMessage(mt, m)

				if err != nil {
					log.Println(err)
					return
				}

				return
			}

			sslCertificatePath := nginx.GetNginxConfPath("ssl/" + domain + "/fullchain.cer")
			_, err = os.Stat(sslCertificatePath)

			if err != nil {
				log.Println(err)
				return
			}

			log.Println("[found]", "fullchain.cer")
			m, err = json.Marshal(gin.H{
				"status":  "success",
				"message": "[found] fullchain.cer",
			})

			if err != nil {
				log.Println(err)
				return
			}

			err = ws.WriteMessage(mt, m)

			if err != nil {
				log.Println(err)
				return
			}

			sslCertificateKeyPath := nginx.GetNginxConfPath("ssl/" + domain + "/" + domain + ".key")
			_, err = os.Stat(sslCertificateKeyPath)

			if err != nil {
				log.Println(err)
				return
			}

			log.Println("[found]", "cert key")
			m, err = json.Marshal(gin.H{
				"status":  "success",
				"message": "[found] cert key",
			})

			if err != nil {
				log.Println(err)
			}

			err = ws.WriteMessage(mt, m)

			if err != nil {
				log.Println(err)
			}

			log.Println("申请成功")
			m, err = json.Marshal(gin.H{
				"status":              "success",
				"message":             "申请成功",
				"ssl_certificate":     sslCertificatePath,
				"ssl_certificate_key": sslCertificateKeyPath,
			})

			if err != nil {
				log.Println(err)
			}

			err = ws.WriteMessage(mt, m)

			if err != nil {
				log.Println(err)
			}
		}
	}
}
