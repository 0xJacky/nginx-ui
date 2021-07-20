package api

import (
    "crypto/tls"
    "encoding/json"
    "github.com/0xJacky/Nginx-UI/tool"
    "github.com/gin-gonic/gin"
    "io"
    "log"
    "net/http"
    "os"
)

func CertInfo(c *gin.Context) {
    domain := c.Param("domain")

    /*sslCertificatePath := tool.GetNginxConfPath("ssl/" + domain +"/fullchain.cer")

    content, err := ioutil.ReadFile(sslCertificatePath)

    if err != nil {
        ErrorHandler(c, err)
        return
    }

    certDERBlock, _ := pem.Decode(content)

    if certDERBlock == nil {
        ErrorHandler(c, errors.New("pem decode error"))
        return
    }

    var key *x509.Certificate
    key, err = x509.ParseCertificate(certDERBlock.Bytes)

    if err != nil {
        ErrorHandler(c, err)
        return
    }*/

    ts := &http.Transport{
        TLSClientConfig: &tls.Config{InsecureSkipVerify: true},
    }

    client := &http.Client{Transport: ts}

    response, err := client.Get("https://" + domain)
    if err != nil {
        ErrorHandler(c, err)
        return
    }
    defer func(Body io.ReadCloser) {
        err = Body.Close()
        if err != nil {
            ErrorHandler(c, err)
            return
        }
    }(response.Body)

    key := response.TLS.PeerCertificates[0]

    c.JSON(http.StatusOK, gin.H{
        "subject_name": key.Subject.CommonName,
        "issuer_name": key.Issuer.CommonName,
        "not_after": key.NotAfter,
        "not_before": key.NotBefore,
    })
}

func IssueCert(c *gin.Context)  {
    domain := c.Param("domain")

    // upgrade http to websocket
    ws, err := upGrader.Upgrade(c.Writer, c.Request, nil)
    if err != nil {
        return
    }

    defer ws.Close()

    for {
        // read
        mt, message, err := ws.ReadMessage()
        if err != nil {
            break
        }
        if string(message) == "go" {
            var m []byte

            err = tool.IssueCert(domain)
            if err != nil {
                m, err = json.Marshal(gin.H{
                    "status": "error",
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

                log.Println(err)
                return
            }

            sslCertificatePath := tool.GetNginxConfPath("ssl/" + domain + "/fullchain.cer")
            _, err = os.Stat(sslCertificatePath)

            if err != nil {
                log.Println(err)
                return
            }

            log.Println("[found]", "fullchain.cer")
            m, err = json.Marshal(gin.H{
                "status": "success",
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

            sslCertificateKeyPath := tool.GetNginxConfPath("ssl/" + domain +"/" + domain + ".key")
            _, err = os.Stat(sslCertificateKeyPath)

            if err != nil {
                log.Println(err)
                return
            }

            log.Println("[found]", "cert key")
            m, err = json.Marshal(gin.H{
                "status": "success",
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
                "status": "success",
                "message": "申请成功",
                "ssl_certificate": sslCertificatePath,
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

