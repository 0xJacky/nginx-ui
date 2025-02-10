package sites

import (
	"github.com/0xJacky/Nginx-UI/internal/cert"
	"github.com/0xJacky/Nginx-UI/internal/nginx"
	"github.com/0xJacky/Nginx-UI/model"
	"github.com/sashabaranov/go-openai"
	"time"
)

type Site struct {
	*model.Site
	Name            string                         `json:"name"`
	ModifiedAt      time.Time                      `json:"modified_at"`
	Enabled         bool                           `json:"enabled"`
	Config          string                         `json:"config"`
	AutoCert        bool                           `json:"auto_cert"`
	ChatGPTMessages []openai.ChatCompletionMessage `json:"chatgpt_messages,omitempty"`
	Tokenized       *nginx.NgxConfig               `json:"tokenized,omitempty"`
	CertInfo        map[int][]*cert.Info           `json:"cert_info,omitempty"`
	Filepath        string                         `json:"filepath"`
}
