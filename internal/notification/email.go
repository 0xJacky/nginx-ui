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
	<body style="font-family: sans-serif; color: #1f2937;">
		<main style="max-width: 640px; margin: 0 auto; padding: 24px;">
			<h1 style="margin: 0 0 16px; color: #1677ff;">{{.Title}}</h1>
			<section style="padding: 16px; background: #f6f8fa; border-radius: 6px;">
				{{.Content}}
			</section>
		</main>
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
	Template string `json:"html_template" title:"HTML Template (Optional)"`
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

		message, err := buildEmailMessage(
			emailConfig.From,
			emailConfig.To,
			msg.GetTitle(n.Language),
			msg.GetContent(n.Language),
			isHTML,
			emailConfig.Template,
		)
		if err != nil {
			return err
		}

		addr := fmt.Sprintf("%s:%s", emailConfig.Host, emailConfig.Port)
		return sendEmail(
			ctx,
			emailConfig.Host,
			emailConfig.Port,
			addr,
			emailConfig.Username,
			emailConfig.Password,
			from.Address,
			to,
			message,
			isSSL,
		)
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

func buildEmailMessage(from, to, subject, content string, html bool, customTemplate string) ([]byte, error) {
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
		templateSource := emailHTMLTemplate
		if strings.TrimSpace(customTemplate) != "" {
			templateSource = customTemplate
		}
		tmpl, err := htmltemplate.New("email").Parse(templateSource)
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

func sendEmail(ctx context.Context, host, port, addr, username, password, from string, to []string, message []byte, ssl bool) error {
	if !ssl {
		return sendEmailPlain(ctx, host, addr, username, password, from, to, message)
	}

	if !isImplicitTLS(port) {
		return sendEmailStartTLS(ctx, host, addr, username, password, from, to, message)
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

	return sendEmailWithClient(client, host, username, password, from, to, message)
}

func isImplicitTLS(port string) bool {
	return strings.TrimSpace(port) == "465"
}

func sendEmailPlain(ctx context.Context, host, addr, username, password, from string, to []string, message []byte) error {
	dialer := &net.Dialer{}
	conn, err := dialer.DialContext(ctx, "tcp", addr)
	if err != nil {
		return err
	}

	client, err := smtp.NewClient(conn, host)
	if err != nil {
		_ = conn.Close()
		return err
	}
	defer client.Close()

	return sendEmailWithClient(client, host, username, password, from, to, message)
}

func sendEmailStartTLS(ctx context.Context, host, addr, username, password, from string, to []string, message []byte) error {
	dialer := &net.Dialer{}
	conn, err := dialer.DialContext(ctx, "tcp", addr)
	if err != nil {
		return err
	}

	client, err := smtp.NewClient(conn, host)
	if err != nil {
		_ = conn.Close()
		return err
	}
	defer client.Close()

	if ok, _ := client.Extension("STARTTLS"); !ok {
		return fmt.Errorf("SMTP server does not support STARTTLS")
	}
	if err := client.StartTLS(&tls.Config{
		ServerName: host,
		MinVersion: tls.VersionTLS12,
	}); err != nil {
		return err
	}

	return sendEmailWithClient(client, host, username, password, from, to, message)
}

func sendEmailWithClient(client *smtp.Client, host, username, password, from string, to []string, message []byte) error {
	if username != "" || password != "" {
		auth := smtpAuthForClient(client, host, username, password)
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

func smtpAuthForClient(client *smtp.Client, host, username, password string) smtp.Auth {
	_, mechanisms := client.Extension("AUTH")
	if supportsSMTPAuth(mechanisms, "LOGIN") {
		return &loginAuth{username: username, password: password}
	}
	if supportsSMTPAuth(mechanisms, "CRAM-MD5") {
		return smtp.CRAMMD5Auth(username, password)
	}

	return smtp.PlainAuth("", username, password, host)
}

func supportsSMTPAuth(mechanisms, mechanism string) bool {
	for _, supported := range strings.Fields(mechanisms) {
		if strings.EqualFold(supported, mechanism) {
			return true
		}
	}

	return false
}

type loginAuth struct {
	username string
	password string
	step     int
}

func (auth *loginAuth) Start(*smtp.ServerInfo) (string, []byte, error) {
	return "LOGIN", nil, nil
}

func (auth *loginAuth) Next(_ []byte, more bool) ([]byte, error) {
	if !more {
		return nil, nil
	}

	switch auth.step {
	case 0:
		auth.step++
		return []byte(auth.username), nil
	case 1:
		auth.step++
		return []byte(auth.password), nil
	default:
		return nil, fmt.Errorf("unexpected LOGIN authentication challenge")
	}
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
