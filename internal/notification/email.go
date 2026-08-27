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
	Host       string `json:"host" title:"Host"`
	Port       string `json:"port" title:"Port"`
	Username   string `json:"username" title:"Username"`
	Password   string `json:"password" title:"Password"`
	From       string `json:"from" title:"From"`
	To         string `json:"to" title:"To"`
	SSL        string `json:"ssl" title:"SSL"`
	Encryption string `json:"encryption" title:"Encryption (none, starttls, opportunistic, tls)"`
	HTML       string `json:"html" title:"HTML"`
	Template   string `json:"html_template" title:"HTML Template (Optional)"`
}

// emailTransport selects how the SMTP connection is encrypted.
type emailTransport int

const (
	// emailTransportPlain never attempts to encrypt the connection.
	emailTransportPlain emailTransport = iota
	// emailTransportOpportunisticSTARTTLS upgrades only when the server
	// advertises STARTTLS and stays plaintext otherwise. This does NOT protect
	// against an attacker stripping the STARTTLS capability.
	emailTransportOpportunisticSTARTTLS
	// emailTransportRequiredSTARTTLS fails closed when the server does not
	// advertise STARTTLS or the upgrade fails.
	emailTransportRequiredSTARTTLS
	// emailTransportImplicitTLS wraps the TCP connection in TLS before any
	// SMTP command is exchanged (SMTPS).
	emailTransportImplicitTLS
)

// parseEmailTransport resolves the explicit encryption mode, falling back to
// the legacy SSL flag so configurations created before this option existed keep
// their original behavior: SSL=true was implicit TLS and SSL=false relied on
// net/smtp.SendMail's opportunistic STARTTLS upgrade.
func parseEmailTransport(raw string, ssl bool) (emailTransport, error) {
	switch strings.ToLower(strings.TrimSpace(raw)) {
	case "":
		if ssl {
			return emailTransportImplicitTLS, nil
		}
		return emailTransportOpportunisticSTARTTLS, nil
	case "none", "plain", "plaintext":
		return emailTransportPlain, nil
	case "starttls":
		return emailTransportRequiredSTARTTLS, nil
	case "starttls-opportunistic", "opportunistic":
		return emailTransportOpportunisticSTARTTLS, nil
	case "tls", "ssl", "implicit", "smtps":
		return emailTransportImplicitTLS, nil
	default:
		return 0, ErrInvalidNotifierConfig
	}
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

		transport, err := parseEmailTransport(emailConfig.Encryption, isSSL)
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
			transport,
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

// sendEmail dispatches to the transport selected by the resolved encryption
// mode. The port is only ever used to dial addr, never to infer the mode.
func sendEmail(ctx context.Context, host, addr, username, password, from string, to []string, message []byte, transport emailTransport) error {
	tlsConfig := &tls.Config{
		ServerName: host,
		MinVersion: tls.VersionTLS12,
	}

	if transport == emailTransportImplicitTLS {
		return sendEmailImplicitTLS(ctx, host, addr, tlsConfig, username, password, from, to, message)
	}

	return sendEmailStartTLS(ctx, host, addr, tlsConfig, username, password, from, to, message, transport)
}

// sendEmailImplicitTLS performs the TLS handshake before the SMTP greeting is
// read, as required by SMTPS servers that never speak plaintext.
func sendEmailImplicitTLS(ctx context.Context, host, addr string, tlsConfig *tls.Config, username, password, from string, to []string, message []byte) error {
	dialer := &net.Dialer{}
	conn, err := dialer.DialContext(ctx, "tcp", addr)
	if err != nil {
		return err
	}

	// Abort the connection if ctx is canceled, since the SMTP operations
	// below do not otherwise observe the context deadline.
	stopWatch := watchContextCancel(ctx, conn)
	defer stopWatch()

	tlsConn := tls.Client(conn, tlsConfig)
	if err := tlsConn.HandshakeContext(ctx); err != nil {
		_ = conn.Close()
		if strings.Contains(err.Error(), "first record does not look like a TLS handshake") {
			return fmt.Errorf("implicit TLS handshake failed, the server likely expects STARTTLS instead: %w", err)
		}
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

// sendEmailStartTLS connects to addr in plaintext and then applies the STARTTLS
// policy implied by transport. Only emailTransportRequiredSTARTTLS fails closed;
// the opportunistic mode stays plaintext when STARTTLS is not advertised and
// therefore does not defend against an attacker stripping that capability.
func sendEmailStartTLS(ctx context.Context, host, addr string, tlsConfig *tls.Config, username, password, from string, to []string, message []byte, transport emailTransport) error {
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

	if transport != emailTransportPlain {
		advertised, _ := client.Extension("STARTTLS")
		if !advertised && transport == emailTransportRequiredSTARTTLS {
			return fmt.Errorf("SMTP server does not support STARTTLS")
		}
		if advertised {
			if err := client.StartTLS(tlsConfig); err != nil {
				return err
			}
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
