package api

import (
    "github.com/0xJacky/Nginx-UI/server/model"
    "github.com/0xJacky/Nginx-UI/server/pkg/cert"
    "github.com/0xJacky/Nginx-UI/server/pkg/config_list"
    "github.com/0xJacky/Nginx-UI/server/pkg/nginx"
    "github.com/gin-gonic/gin"
    "log"
    "net/http"
    "os"
    "path/filepath"
    "strings"
    "time"
)

func GetDomains(c *gin.Context) {
    name := c.Query("name")
    orderBy := c.Query("order_by")
    sort := c.DefaultQuery("sort", "desc")

    mySort := map[string]string{
        "enabled": "bool",
        "name":    "string",
        "modify":  "time",
    }

    configFiles, err := os.ReadDir(nginx.GetNginxConfPath("sites-available"))

    if err != nil {
        ErrHandler(c, err)
        return
    }

    enabledConfig, err := os.ReadDir(filepath.Join(nginx.GetNginxConfPath("sites-enabled")))

    if err != nil {
        ErrHandler(c, err)
        return
    }

    enabledConfigMap := make(map[string]bool)
    for i := range enabledConfig {
        enabledConfigMap[enabledConfig[i].Name()] = true
    }

    var configs []gin.H

    for i := range configFiles {
        file := configFiles[i]
        fileInfo, _ := file.Info()
        if !file.IsDir() {
            if name != "" && !strings.Contains(file.Name(), name) {
                continue
            }
            configs = append(configs, gin.H{
                "name":    file.Name(),
                "size":    fileInfo.Size(),
                "modify":  fileInfo.ModTime(),
                "enabled": enabledConfigMap[file.Name()],
            })
        }
    }

    configs = config_list.Sort(orderBy, sort, mySort[orderBy], configs)

    c.JSON(http.StatusOK, gin.H{
        "data": configs,
    })
}

type CertificateInfo struct {
    SubjectName string    `json:"subject_name"`
    IssuerName  string    `json:"issuer_name"`
    NotAfter    time.Time `json:"not_after"`
    NotBefore   time.Time `json:"not_before"`
}

func GetDomain(c *gin.Context) {
    rewriteName, ok := c.Get("rewriteConfigFileName")

    name := c.Param("name")

    // for modify filename
    if ok {
        name = rewriteName.(string)
    }

    path := filepath.Join(nginx.GetNginxConfPath("sites-available"), name)

    enabled := true
    if _, err := os.Stat(filepath.Join(nginx.GetNginxConfPath("sites-enabled"), name)); os.IsNotExist(err) {
        enabled = false
    }

    c.Set("maybe_error", "nginx_config_syntax_error")
    config, err := nginx.ParseNgxConfig(path)

    if err != nil {
        ErrHandler(c, err)
        return
    }

    c.Set("maybe_error", "")

    certInfoMap := make(map[int]CertificateInfo)
    var serverName string
    for serverIdx, server := range config.Servers {
        for _, directive := range server.Directives {

            if directive.Directive == "server_name" {
                serverName = strings.ReplaceAll(directive.Params, " ", "_")
                continue
            }

            if directive.Directive == "ssl_certificate" {

                pubKey, err := cert.GetCertInfo(directive.Params)

                if err != nil {
                    log.Println("Failed to get certificate information", err)
                    break
                }

                certInfoMap[serverIdx] = CertificateInfo{
                    SubjectName: pubKey.Subject.CommonName,
                    IssuerName:  pubKey.Issuer.CommonName,
                    NotAfter:    pubKey.NotAfter,
                    NotBefore:   pubKey.NotBefore,
                }

                break
            }
        }
    }

    certModel, _ := model.FirstCert(serverName)

    c.Set("maybe_error", "nginx_config_syntax_error")

    c.JSON(http.StatusOK, gin.H{
        "enabled":   enabled,
        "name":      name,
        "config":    config.FmtCode(),
        "tokenized": config,
        "auto_cert": certModel.AutoCert == model.AutoCertEnabled,
        "cert_info": certInfoMap,
    })

}

func EditDomain(c *gin.Context) {
    name := c.Param("name")

    if name == "" {
        c.JSON(http.StatusNotAcceptable, gin.H{
            "message": "param name is empty",
        })
        return
    }

    var json struct {
        Name    string `json:"name" binding:"required"`
        Content string `json:"content" binding:"required"`
    }

    if !BindAndValid(c, &json) {
        return
    }

    path := filepath.Join(nginx.GetNginxConfPath("sites-available"), name)

    err := os.WriteFile(path, []byte(json.Content), 0644)
    if err != nil {
        ErrHandler(c, err)
        return
    }
    enabledConfigFilePath := filepath.Join(nginx.GetNginxConfPath("sites-enabled"), name)
    // rename the config file if needed
    if name != json.Name {
        newPath := filepath.Join(nginx.GetNginxConfPath("sites-available"), json.Name)
        // recreate soft link
        log.Println(enabledConfigFilePath)
        if _, err = os.Stat(enabledConfigFilePath); err == nil {
            log.Println(enabledConfigFilePath)
            _ = os.Remove(enabledConfigFilePath)
            enabledConfigFilePath = filepath.Join(nginx.GetNginxConfPath("sites-enabled"), json.Name)
            err = os.Symlink(newPath, enabledConfigFilePath)

            if err != nil {
                ErrHandler(c, err)
                return
            }
        }
        err = os.Rename(path, newPath)
        if err != nil {
            ErrHandler(c, err)
            return
        }
        name = json.Name
        c.Set("rewriteConfigFileName", name)

    }

    enabledConfigFilePath = filepath.Join(nginx.GetNginxConfPath("sites-enabled"), name)
    if _, err = os.Stat(enabledConfigFilePath); err == nil {
        // Test nginx configuration
        err = nginx.TestNginxConf()
        if err != nil {
            c.JSON(http.StatusInternalServerError, gin.H{
                "message": err.Error(),
                "error":   "nginx_config_syntax_error",
            })
            return
        }

        output := nginx.ReloadNginx()

        if output != "" && strings.Contains(output, "error") {
            c.JSON(http.StatusInternalServerError, gin.H{
                "message": output,
            })
            return
        }
    }

    GetDomain(c)
}

func EnableDomain(c *gin.Context) {
    configFilePath := filepath.Join(nginx.GetNginxConfPath("sites-available"), c.Param("name"))
    enabledConfigFilePath := filepath.Join(nginx.GetNginxConfPath("sites-enabled"), c.Param("name"))

    _, err := os.Stat(configFilePath)

    if err != nil {
        ErrHandler(c, err)
        return
    }

    if _, err = os.Stat(enabledConfigFilePath); os.IsNotExist(err) {
        err = os.Symlink(configFilePath, enabledConfigFilePath)

        if err != nil {
            ErrHandler(c, err)
            return
        }
    }

    // Test nginx config, if not pass then rollback.
    err = nginx.TestNginxConf()
    if err != nil {
        _ = os.Remove(enabledConfigFilePath)
        c.JSON(http.StatusInternalServerError, gin.H{
            "message": err.Error(),
        })
        return
    }

    output := nginx.ReloadNginx()

    if output != "" && strings.Contains(output, "error") {
        c.JSON(http.StatusInternalServerError, gin.H{
            "message": output,
        })
        return
    }

    c.JSON(http.StatusOK, gin.H{
        "message": "ok",
    })
}

func DisableDomain(c *gin.Context) {
    enabledConfigFilePath := filepath.Join(nginx.GetNginxConfPath("sites-enabled"), c.Param("name"))

    _, err := os.Stat(enabledConfigFilePath)

    if err != nil {
        ErrHandler(c, err)
        return
    }

    err = os.Remove(enabledConfigFilePath)

    if err != nil {
        ErrHandler(c, err)
        return
    }

    // delete auto cert record
    certModel := model.Cert{Domain: c.Param("name")}
    err = certModel.Remove()
    if err != nil {
        ErrHandler(c, err)
        return
    }

    output := nginx.ReloadNginx()

    if output != "" {
        c.JSON(http.StatusInternalServerError, gin.H{
            "message": output,
        })
        return
    }

    c.JSON(http.StatusOK, gin.H{
        "message": "ok",
    })
}

func DeleteDomain(c *gin.Context) {
    var err error
    name := c.Param("name")
    availablePath := filepath.Join(nginx.GetNginxConfPath("sites-available"), name)
    enabledPath := filepath.Join(nginx.GetNginxConfPath("sites-enabled"), name)

    if _, err = os.Stat(availablePath); os.IsNotExist(err) {
        c.JSON(http.StatusNotFound, gin.H{
            "message": "site not found",
        })
        return
    }

    if _, err = os.Stat(enabledPath); err == nil {
        c.JSON(http.StatusNotAcceptable, gin.H{
            "message": "site is enabled",
        })
        return
    }

    certModel := model.Cert{Domain: name}
    _ = certModel.Remove()

    err = os.Remove(availablePath)

    if err != nil {
        ErrHandler(c, err)
        return
    }

    c.JSON(http.StatusOK, gin.H{
        "message": "ok",
    })

}

func AddDomainToAutoCert(c *gin.Context) {
    domain := c.Param("domain")
    domain = strings.ReplaceAll(domain, " ", "_")
    certModel, err := model.FirstOrCreateCert(domain)

    if err != nil {
        ErrHandler(c, err)
        return
    }

    err = certModel.Updates(&model.Cert{
        AutoCert: model.AutoCertEnabled,
    })

    if err != nil {
        ErrHandler(c, err)
        return
    }

    c.JSON(http.StatusOK, certModel)
}

func RemoveDomainFromAutoCert(c *gin.Context) {
    domain := c.Param("domain")
    domain = strings.ReplaceAll(domain, " ", "_")
    certModel := model.Cert{
        Domain: domain,
    }

    err := certModel.Updates(&model.Cert{
        AutoCert: model.AutoCertDisabled,
    })

    if err != nil {
        ErrHandler(c, err)
        return
    }
    c.JSON(http.StatusOK, nil)
}
