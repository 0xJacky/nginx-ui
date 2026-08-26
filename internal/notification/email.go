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

// Email holds the external_notify configuration for the built-in email notifier.
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

// emailMessageData is the data passed to the HTML email template.
type emailMessageData struct {
	Title   string
	Content any
}

func init() {
	// RegisterExternalNotifier wires the "email" notifier into the generic
	// external notification dispatch used across the app.
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

		addr := net.JoinHostPort(emailConfig.Host, emailConfig.Port)
		return sendEmail(
			ctx,
			emailConfig.Host,
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

// parseOptionalBool treats an empty string as false instead of an error,
// since the config value is stored as free-form text and may be unset.
func parseOptionalBool(raw string) (bool, error) {
	value := strings.TrimSpace(raw)
	if value == "" {
		return false, nil
	}

	return strconv.ParseBool(value)
}

// parseEmailRecipients accepts a comma-separated recipient list and returns
// only the bare addresses (display names are dropped).
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

// buildEmailMessage renders the RFC 5322 message (headers + body) ready to be
// streamed to an SMTP DATA command, optionally wrapping content in the
// built-in or a user-supplied HTML template.
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

// sendEmail dispatches to the STARTTLS transport. SSL alone decides whether
// encryption is required; the port is never used to infer implicit TLS vs
// STARTTLS, and the connection always dials the configured host:port as-is.
func sendEmail(ctx context.Context, host, addr, username, password, from string, to []string, message []byte, ssl bool) error {
	// ssl=true requires the server to support STARTTLS and fails otherwise;
	// ssl=false still upgrades opportunistically if STARTTLS is advertised,
	// so a downgrade attack cannot silently force plaintext.
	return sendEmailStartTLS(ctx, host, addr, username, password, from, to, message, ssl)
}

// sendEmailStartTLS always connects to addr in plaintext first (the port is
// used exactly as configured). When requireTLS is true (SSL enabled) it
// demands the server support STARTTLS and fails otherwise; when false it
// upgrades opportunistically whenever the server advertises STARTTLS,
// mirroring net/smtp.SendMail so a downgrade attack cannot silently disable
// encryption.
func sendEmailStartTLS(ctx context.Context, host, addr, username, password, from string, to []string, message []byte, requireTLS bool) error {
	dialer := &net.Dialer{}
	conn, err := dialer.DialContext(ctx, "tcp", addr)
	if err != nil {
		return err
	}

	// Abort the connection if ctx is canceled, since the SMTP operations
	// below do not otherwise observe the context deadline.
	stopWatch := watchContextCancel(ctx, conn)
	defer stopWatch()

	client, err := smtp.NewClient(conn, host)
	if err != nil {
		_ = conn.Close()
		return err
	}
	defer client.Close()

	ok, _ := client.Extension("STARTTLS")
	if !ok && requireTLS {
		return fmt.Errorf("SMTP server does not support STARTTLS")
	}
	if ok {
		if err := client.StartTLS(&tls.Config{
			ServerName: host,
			MinVersion: tls.VersionTLS12,
		}); err != nil {
			return err
		}
	}

	return sendEmailWithClient(client, host, username, password, from, to, message)
}

// watchContextCancel closes conn once ctx is done, unblocking any SMTP I/O
// that net/smtp performs without context awareness. Call the returned stop
// function once the connection is no longer needed to release the goroutine.
func watchContextCancel(ctx context.Context, conn net.Conn) (stop func()) {
	done := make(chan struct{})
	go func() {
		select {
		case <-ctx.Done():
			_ = conn.Close()
		case <-done:
		}
	}()

	return func() { close(done) }
}

// sendEmailWithClient runs the AUTH/MAIL/RCPT/DATA/QUIT sequence on an
// already-connected (and, if applicable, already-upgraded-to-TLS) client.
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

// smtpAuthForClient picks the strongest AUTH mechanism the server advertises,
// preferring LOGIN/CRAM-MD5 over PLAIN, and falls back to PLAIN otherwise.
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

// supportsSMTPAuth reports whether mechanism is present in the space-separated
// AUTH capability list advertised by the server.
func supportsSMTPAuth(mechanisms, mechanism string) bool {
	for _, supported := range strings.Fields(mechanisms) {
		if strings.EqualFold(supported, mechanism) {
			return true
		}
	}

	return false
}

// loginAuth implements the SMTP AUTH LOGIN mechanism, which net/smtp does not
// provide natively (only PLAIN and CRAM-MD5 are built in).
type loginAuth struct {
	username string
	password string
	step     int
}

func (auth *loginAuth) Start(server *smtp.ServerInfo) (string, []byte, error) {
	// Mirror smtp.PlainAuth's safeguard: refuse to hand over credentials
	// unless the connection is encrypted or talking to localhost.
	if !server.TLS && !isLocalhostAddr(server.Name) {
		return "", nil, fmt.Errorf("unencrypted connection")
	}

	return "LOGIN", nil, nil
}

// Next replies to the server's username/password challenges in order.
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

// formatEmailAddresses joins parsed addresses back into a single header value.
func formatEmailAddresses(addresses []*mail.Address) string {
	formatted := make([]string, 0, len(addresses))
	for _, address := range addresses {
		formatted = append(formatted, address.String())
	}

	return strings.Join(formatted, ", ")
}

// sanitizeEmailHeader strips CR/LF to prevent header/SMTP command injection
// via user-controlled notification titles.
func sanitizeEmailHeader(value string) string {
	value = strings.ReplaceAll(value, "\r", "")
	return strings.ReplaceAll(value, "\n", "")
}

// isLocalhostAddr reports whether name is a loopback host, matching the
// check net/smtp.PlainAuth performs before allowing plaintext credentials.
func isLocalhostAddr(name string) bool {
	return name == "localhost" || name == "127.0.0.1" || name == "::1"
}
