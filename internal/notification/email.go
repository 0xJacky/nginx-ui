package notification

import (
	"bytes"
	"context"
	"crypto/tls"
	"fmt"
	htmltemplate "html/template"
	"mime"
	"net"
	"net/mail"
	"net/smtp"
	"strconv"
	"strings"

	"github.com/0xJacky/Nginx-UI/model"
	"github.com/uozi-tech/cosy/map2struct"
)

const emailHTMLTemplate = `<!doctype html>
<html>
<head>
  <meta charset="UTF-8">
  <title>{{.Title}}</title>
</head>
<body>
  <h2>{{.Title}}</h2>
  <div>{{.Content}}</div>
</body>
</html>`

// @external_notifier(Email)
type Email struct {
	Host     string `json:"host" title:"Host"`
	Port     string `json:"port" title:"Port"`
	Username string `json:"username" title:"Username"`
	Password string `json:"password" title:"Password"`
	From     string `json:"from" title:"From"`
	To       string `json:"to" title:"To"`
	SSL      string `json:"ssl" title:"SSL"`
	HTML     string `json:"html" title:"HTML"`
}

type emailMessageData struct {
	Title   string
	Content any
}

func init() {
	RegisterExternalNotifier("email", func(ctx context.Context, n *model.ExternalNotify, msg *ExternalMessage) error {
		emailConfig := &Email{}
		err := map2struct.WeakDecode(n.Config, emailConfig)
		if err != nil {
			return err
		}
		if emailConfig.Host == "" || emailConfig.Port == "" || emailConfig.From == "" || emailConfig.To == "" {
			return ErrInvalidNotifierConfig
		}

		to, err := parseEmailRecipients(emailConfig.To)
		if err != nil || len(to) == 0 {
			return ErrInvalidNotifierConfig
		}

		from, err := mail.ParseAddress(emailConfig.From)
		if err != nil {
			return ErrInvalidNotifierConfig
		}

		isSSL, err := parseOptionalBool(emailConfig.SSL)
		if err != nil {
			return ErrInvalidNotifierConfig
		}

		isHTML, err := parseOptionalBool(emailConfig.HTML)
		if err != nil {
			return ErrInvalidNotifierConfig
		}

		message, err := buildEmailMessage(emailConfig.From, emailConfig.To, msg.GetTitle(n.Language), msg.GetContent(n.Language), isHTML)
		if err != nil {
			return err
		}

		addr := fmt.Sprintf("%s:%s", emailConfig.Host, emailConfig.Port)
		var auth smtp.Auth
		if emailConfig.Username != "" || emailConfig.Password != "" {
			auth = smtp.PlainAuth("", emailConfig.Username, emailConfig.Password, emailConfig.Host)
		}

		return sendEmail(ctx, emailConfig.Host, addr, auth, from.Address, to, message, isSSL)
	})
}

func parseOptionalBool(raw string) (bool, error) {
	value := strings.TrimSpace(raw)
	if value == "" {
		return false, nil
	}

	return strconv.ParseBool(value)
}

func parseEmailRecipients(raw string) ([]string, error) {
	addresses, err := mail.ParseAddressList(raw)
	if err != nil {
		return nil, err
	}

	recipients := make([]string, 0, len(addresses))
	for _, address := range addresses {
		recipients = append(recipients, address.Address)
	}

	return recipients, nil
}

func buildEmailMessage(from, to, subject, content string, html bool) ([]byte, error) {
	fromAddress, err := mail.ParseAddress(from)
	if err != nil {
		return nil, err
	}
	toAddresses, err := mail.ParseAddressList(to)
	if err != nil {
		return nil, err
	}

	contentType := "text/plain; charset=UTF-8"
	body := fmt.Sprintf("%s\n\n%s", subject, content)
	if html {
		contentType = "text/html; charset=UTF-8"
		var htmlBody bytes.Buffer
		tmpl, err := htmltemplate.New("email").Parse(emailHTMLTemplate)
		if err != nil {
			return nil, err
		}
		err = tmpl.Execute(&htmlBody, emailMessageData{
			Title:   subject,
			Content: htmltemplate.HTML(strings.ReplaceAll(htmltemplate.HTMLEscapeString(content), "\n", "<br>\n")),
		})
		if err != nil {
			return nil, err
		}
		body = htmlBody.String()
	}

	headers := map[string]string{
		"From":         fromAddress.String(),
		"To":           formatEmailAddresses(toAddresses),
		"Subject":      mime.QEncoding.Encode("UTF-8", sanitizeEmailHeader(subject)),
		"MIME-Version": "1.0",
		"Content-Type": contentType,
	}

	var message bytes.Buffer
	for _, key := range []string{"From", "To", "Subject", "MIME-Version", "Content-Type"} {
		message.WriteString(key)
		message.WriteString(": ")
		message.WriteString(headers[key])
		message.WriteString("\r\n")
	}
	message.WriteString("\r\n")
	message.WriteString(body)

	return message.Bytes(), nil
}

func sendEmail(ctx context.Context, host, addr string, auth smtp.Auth, from string, to []string, message []byte, ssl bool) error {
	if !ssl {
		done := make(chan error, 1)
		go func() {
			done <- smtp.SendMail(addr, auth, from, to, message)
		}()

		select {
		case <-ctx.Done():
			return ctx.Err()
		case err := <-done:
			return err
		}
	}

	dialer := &net.Dialer{}
	conn, err := dialer.DialContext(ctx, "tcp", addr)
	if err != nil {
		return err
	}

	tlsConn := tls.Client(conn, &tls.Config{
		ServerName: host,
		MinVersion: tls.VersionTLS12,
	})
	if err := tlsConn.HandshakeContext(ctx); err != nil {
		_ = conn.Close()
		return err
	}

	client, err := smtp.NewClient(tlsConn, host)
	if err != nil {
		_ = tlsConn.Close()
		return err
	}
	defer client.Close()

	if auth != nil {
		if err := client.Auth(auth); err != nil {
			return err
		}
	}
	if err := client.Mail(from); err != nil {
		return err
	}
	for _, recipient := range to {
		if err := client.Rcpt(recipient); err != nil {
			return err
		}
	}
	writer, err := client.Data()
	if err != nil {
		return err
	}
	if _, err := writer.Write(message); err != nil {
		_ = writer.Close()
		return err
	}
	if err := writer.Close(); err != nil {
		return err
	}

	return client.Quit()
}

func formatEmailAddresses(addresses []*mail.Address) string {
	formatted := make([]string, 0, len(addresses))
	for _, address := range addresses {
		formatted = append(formatted, address.String())
	}

	return strings.Join(formatted, ", ")
}

func sanitizeEmailHeader(value string) string {
	value = strings.ReplaceAll(value, "\r", "")
	return strings.ReplaceAll(value, "\n", "")
}
