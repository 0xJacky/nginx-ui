package cert

import (
	"fmt"
	"github.com/uozi-tech/cosy/logger"
	"github.com/0xJacky/Nginx-UI/model"
	"strings"
	"time"
)

type Logger struct {
	buffer []string
	cert   *model.Cert
}

func (t *Logger) SetCertModel(cert *model.Cert) {
	t.cert = cert
}

func (t *Logger) Info(text string) {
	t.buffer = append(t.buffer, strings.TrimSpace(text))
	logger.Info("AutoCert", strings.TrimSpace(text))
}

func (t *Logger) Error(err error) {
	t.buffer = append(t.buffer, fmt.Sprintf("%s [Error] %s",
		time.Now().Format("2006/01/02 15:04:05"),
		strings.TrimSpace(err.Error()),
	))
	logger.Error("AutoCert", err)
}

func (t *Logger) Exit() {
	if t.cert == nil {
		return
	}

	_ = t.cert.Updates(&model.Cert{
		Log: t.ToString(),
	})
}

func (t *Logger) ToString() (content string) {
	content = strings.Join(t.buffer, "\n")
	return
}
