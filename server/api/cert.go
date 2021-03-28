package api

import (
    "bytes"
    "encoding/json"
    "github.com/0xJacky/Nginx-UI/tool"
    "github.com/gin-gonic/gin"
    "log"
    "os"
    "os/exec"
)

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
        if string(message) == "ping" {
            var m []byte
            cmdOutput := bytes.NewBuffer(nil)
            cmd := exec.Command("bash",  "/usr/local/acme.sh/acme.sh",
                "--issue",
                "-d", domain,
                "--nginx", "--force", "--log")

            cmd.Stdout = cmdOutput
            cmd.Stderr = cmdOutput

            err := cmd.Run()

            if err != nil {
                log.Println(err)
                m, err = json.Marshal(gin.H{
                    "status": "error",
                    "message": err.Error(),
                })

                if err != nil {
                    log.Println(err)
                }

                err = ws.WriteMessage(mt, m)

                if err != nil {
                    log.Println(err)
                }
            }

            m, err = json.Marshal(gin.H{
                "status": "info",
                "message": cmdOutput.String(),
            })

            if err != nil {
                log.Println(err)
            }

            err = ws.WriteMessage(mt, m)

            if err != nil {
                log.Println(err)
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
            }

            err = ws.WriteMessage(mt, m)

            if err != nil {
                log.Println(err)
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
