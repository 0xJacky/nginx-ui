package api

import (
    "github.com/0xJacky/Nginx-UI/internal/cert"
    "github.com/0xJacky/Nginx-UI/internal/config_list"
    helper2 "github.com/0xJacky/Nginx-UI/internal/helper"
    "github.com/0xJacky/Nginx-UI/internal/logger"
    nginx2 "github.com/0xJacky/Nginx-UI/internal/nginx"
    "github.com/0xJacky/Nginx-UI/model"
    "github.com/0xJacky/Nginx-UI/query"
    "github.com/gin-gonic/gin"
    "github.com/sashabaranov/go-openai"
    "net/http"
    "os"
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

	configFiles, err := os.ReadDir(nginx2.GetConfPath("sites-available"))

	if err != nil {
		ErrHandler(c, err)
		return
	}

	enabledConfig, err := os.ReadDir(nginx2.GetConfPath("sites-enabled"))

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

	path := nginx2.GetConfPath("sites-available", name)
	file, err := os.Stat(path)
	if os.IsNotExist(err) {
		c.JSON(http.StatusNotFound, gin.H{
			"message": "file not found",
		})
		return
	}

	enabled := true

	if _, err := os.Stat(nginx2.GetConfPath("sites-enabled", name)); os.IsNotExist(err) {
		enabled = false
	}

	g := query.ChatGPTLog
	chatgpt, err := g.Where(g.Name.Eq(path)).FirstOrCreate()

	if err != nil {
		ErrHandler(c, err)
		return
	}

	if chatgpt.Content == nil {
		chatgpt.Content = make([]openai.ChatCompletionMessage, 0)
	}

	s := query.Site
	site, err := s.Where(s.Path.Eq(path)).FirstOrInit()

	if err != nil {
		ErrHandler(c, err)
		return
	}

	certModel, err := model.FirstCert(name)

	if err != nil {
		logger.Warn("cert", err)
	}

	if site.Advanced {
		origContent, err := os.ReadFile(path)
		if err != nil {
			ErrHandler(c, err)
			return
		}

		c.JSON(http.StatusOK, gin.H{
			"modified_at":      file.ModTime(),
			"advanced":         site.Advanced,
			"enabled":          enabled,
			"name":             name,
			"config":           string(origContent),
			"auto_cert":        certModel.AutoCert == model.AutoCertEnabled,
			"chatgpt_messages": chatgpt.Content,
		})
		return
	}

	c.Set("maybe_error", "nginx_config_syntax_error")
	config, err := nginx2.ParseNgxConfig(path)

	if err != nil {
		ErrHandler(c, err)
		return
	}

	c.Set("maybe_error", "")

	certInfoMap := make(map[int]CertificateInfo)
	for serverIdx, server := range config.Servers {
		for _, directive := range server.Directives {
			if directive.Directive == "ssl_certificate" {

				pubKey, err := cert.GetCertInfo(directive.Params)

				if err != nil {
					logger.Error("Failed to get certificate information", err)
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

	c.Set("maybe_error", "nginx_config_syntax_error")

	c.JSON(http.StatusOK, gin.H{
		"modified_at":      file.ModTime(),
		"advanced":         site.Advanced,
		"enabled":          enabled,
		"name":             name,
		"config":           config.FmtCode(),
		"tokenized":        config,
		"auto_cert":        certModel.AutoCert == model.AutoCertEnabled,
		"cert_info":        certInfoMap,
		"chatgpt_messages": chatgpt.Content,
	})

}

func SaveDomain(c *gin.Context) {
	name := c.Param("name")

	if name == "" {
		c.JSON(http.StatusNotAcceptable, gin.H{
			"message": "param name is empty",
		})
		return
	}

	var json struct {
		Name      string `json:"name" binding:"required"`
		Content   string `json:"content" binding:"required"`
		Overwrite bool   `json:"overwrite"`
	}

	if !BindAndValid(c, &json) {
		return
	}

	path := nginx2.GetConfPath("sites-available", name)

	if !json.Overwrite && helper2.FileExists(path) {
		c.JSON(http.StatusNotAcceptable, gin.H{
			"message": "File exists",
		})
		return
	}

	err := os.WriteFile(path, []byte(json.Content), 0644)
	if err != nil {
		ErrHandler(c, err)
		return
	}
	enabledConfigFilePath := nginx2.GetConfPath("sites-enabled", name)
	// rename the config file if needed
	if name != json.Name {
		newPath := nginx2.GetConfPath("sites-available", json.Name)
		s := query.Site
		_, err = s.Where(s.Path.Eq(path)).Update(s.Path, newPath)

		// check if dst file exists, do not rename
		if helper2.FileExists(newPath) {
			c.JSON(http.StatusNotAcceptable, gin.H{
				"message": "File exists",
			})
			return
		}
		// recreate soft link
		if helper2.FileExists(enabledConfigFilePath) {
			_ = os.Remove(enabledConfigFilePath)
			enabledConfigFilePath = nginx2.GetConfPath("sites-enabled", json.Name)
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

	enabledConfigFilePath = nginx2.GetConfPath("sites-enabled", name)
	if helper2.FileExists(enabledConfigFilePath) {
		// Test nginx configuration
		output := nginx2.TestConf()

		if nginx2.GetLogLevel(output) > nginx2.Warn {
			c.JSON(http.StatusInternalServerError, gin.H{
				"message": output,
				"error":   "nginx_config_syntax_error",
			})
			return
		}

		output = nginx2.Reload()

		if nginx2.GetLogLevel(output) > nginx2.Warn {
			c.JSON(http.StatusInternalServerError, gin.H{
				"message": output,
			})
			return
		}
	}

	GetDomain(c)
}

func EnableDomain(c *gin.Context) {
	configFilePath := nginx2.GetConfPath("sites-available", c.Param("name"))
	enabledConfigFilePath := nginx2.GetConfPath("sites-enabled", c.Param("name"))

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

	// Test nginx config, if not pass then disable the site.
	output := nginx2.TestConf()

	if nginx2.GetLogLevel(output) > nginx2.Warn {
		_ = os.Remove(enabledConfigFilePath)
		c.JSON(http.StatusInternalServerError, gin.H{
			"message": output,
		})
		return
	}

	output = nginx2.Reload()

	if nginx2.GetLogLevel(output) > nginx2.Warn {
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
	enabledConfigFilePath := nginx2.GetConfPath("sites-enabled", c.Param("name"))

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
	certModel := model.Cert{Filename: c.Param("name")}
	err = certModel.Remove()
	if err != nil {
		ErrHandler(c, err)
		return
	}

	output := nginx2.Reload()

	if nginx2.GetLogLevel(output) > nginx2.Warn {
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
	availablePath := nginx2.GetConfPath("sites-available", name)
	enabledPath := nginx2.GetConfPath("sites-enabled", name)

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

	certModel := model.Cert{Filename: name}
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
	name := c.Param("name")

	var json struct {
        model.Cert
		Domains []string `json:"domains"`
	}

	if !BindAndValid(c, &json) {
		return
	}

	certModel, err := model.FirstOrCreateCert(name)

	if err != nil {
		ErrHandler(c, err)
		return
	}

	err = certModel.Updates(&model.Cert{
		Name:            name,
		Domains:         json.Domains,
		AutoCert:        model.AutoCertEnabled,
		DnsCredentialID: json.DnsCredentialID,
		ChallengeMethod: json.ChallengeMethod,
	})

	if err != nil {
		ErrHandler(c, err)
		return
	}

	c.JSON(http.StatusOK, certModel)
}

func RemoveDomainFromAutoCert(c *gin.Context) {
	name := c.Param("name")
	certModel, err := model.FirstCert(name)

	if err != nil {
		ErrHandler(c, err)
		return
	}

	err = certModel.Updates(&model.Cert{
		AutoCert: model.AutoCertDisabled,
	})

	if err != nil {
		ErrHandler(c, err)
		return
	}
	c.JSON(http.StatusOK, nil)
}

func DuplicateSite(c *gin.Context) {
	name := c.Param("name")

	var json struct {
		Name string `json:"name" binding:"required"`
	}

	if !BindAndValid(c, &json) {
		return
	}

	src := nginx2.GetConfPath("sites-available", name)
	dst := nginx2.GetConfPath("sites-available", json.Name)

	if helper2.FileExists(dst) {
		c.JSON(http.StatusNotAcceptable, gin.H{
			"message": "File exists",
		})
		return
	}

	_, err := helper2.CopyFile(src, dst)

	if err != nil {
		ErrHandler(c, err)
		return
	}

	c.JSON(http.StatusOK, gin.H{
		"dst": dst,
	})
}

func DomainEditByAdvancedMode(c *gin.Context) {
	var json struct {
		Advanced bool `json:"advanced"`
	}

	if !BindAndValid(c, &json) {
		return
	}

	name := c.Param("name")
	path := nginx2.GetConfPath("sites-available", name)

	s := query.Site

	_, err := s.Where(s.Path.Eq(path)).FirstOrCreate()
	if err != nil {
		ErrHandler(c, err)
		return
	}

	_, err = s.Where(s.Path.Eq(path)).Update(s.Advanced, json.Advanced)

	if err != nil {
		ErrHandler(c, err)
		return
	}

	c.JSON(http.StatusOK, gin.H{
		"message": "ok",
	})

}
