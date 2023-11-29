package sites

import (
	"github.com/0xJacky/Nginx-UI/internal/nginx"
	"github.com/sashabaranov/go-openai"
	"time"
)

type CertificateInfo struct {
	SubjectName string    `json:"subject_name"`
	IssuerName  string    `json:"issuer_name"`
	NotAfter    time.Time `json:"not_after"`
	NotBefore   time.Time `json:"not_before"`
}

type Site struct {
	ModifiedAt      time.Time                      `json:"modified_at"`
	Advanced        bool                           `json:"advanced"`
	Enabled         bool                           `json:"enabled"`
	Name            string                         `json:"name"`
	Config          string                         `json:"config"`
	AutoCert        bool                           `json:"auto_cert"`
	ChatGPTMessages []openai.ChatCompletionMessage `json:"chatgpt_messages,omitempty"`
	Tokenized       *nginx.NgxConfig               `json:"tokenized,omitempty"`
	CertInfo        map[int]CertificateInfo        `json:"cert_info,omitempty"`
}
