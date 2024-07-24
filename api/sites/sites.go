package sites

import (
	"github.com/0xJacky/Nginx-UI/internal/cert"
	"github.com/0xJacky/Nginx-UI/internal/nginx"
	"github.com/sashabaranov/go-openai"
	"time"
)

type Site struct {
	ModifiedAt      time.Time                      `json:"modified_at"`
	Advanced        bool                           `json:"advanced"`
	Enabled         bool                           `json:"enabled"`
	Name            string                         `json:"name"`
	Config          string                         `json:"config"`
	AutoCert        bool                           `json:"auto_cert"`
	ChatGPTMessages []openai.ChatCompletionMessage `json:"chatgpt_messages,omitempty"`
	Tokenized       *nginx.NgxConfig               `json:"tokenized,omitempty"`
	CertInfo        map[int][]*cert.Info           `json:"cert_info,omitempty"`
	Filepath        string                         `json:"filepath"`
}
