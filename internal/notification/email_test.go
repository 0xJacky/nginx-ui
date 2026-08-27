package notification

import (
	"bufio"
	"context"
	"crypto/ecdsa"
	"crypto/elliptic"
	"crypto/rand"
	"crypto/tls"
	"crypto/x509"
	"crypto/x509/pkix"
	"math/big"
	"net"
	"net/smtp"
	"reflect"
	"strings"
	"sync"
	"testing"
	"time"
)

func TestParseEmailRecipientsSupportsCommaSeparatedList(t *testing.T) {
	recipients, err := parseEmailRecipients("first@example.com, Second <second@example.com>")
	if err != nil {
		t.Fatalf("parseEmailRecipients() error = %v", err)
	}

	want := []string{"first@example.com", "second@example.com"}
	if !reflect.DeepEqual(recipients, want) {
		t.Fatalf("parseEmailRecipients() = %#v, want %#v", recipients, want)
	}
}

func TestParseEmailTransportFallsBackToLegacySSLFlag(t *testing.T) {
	tests := []struct {
		raw  string
		ssl  bool
		want emailTransport
	}{
		{raw: "", ssl: true, want: emailTransportImplicitTLS},
		{raw: "", ssl: false, want: emailTransportOpportunisticSTARTTLS},
		{raw: "none", ssl: true, want: emailTransportPlain},
		{raw: "STARTTLS", ssl: false, want: emailTransportRequiredSTARTTLS},
		{raw: " tls ", ssl: false, want: emailTransportImplicitTLS},
	}

	for _, tt := range tests {
		got, err := parseEmailTransport(tt.raw, tt.ssl)
		if err != nil {
			t.Fatalf("parseEmailTransport(%q, %v) error = %v", tt.raw, tt.ssl, err)
		}
		if got != tt.want {
			t.Fatalf("parseEmailTransport(%q, %v) = %v, want %v", tt.raw, tt.ssl, got, tt.want)
		}
	}

	if _, err := parseEmailTransport("bogus", false); err == nil {
		t.Fatal("parseEmailTransport(bogus) error = nil, want error")
	}
}

// TestSendEmailStartTLSRequiredFailsClosed verifies that a required STARTTLS
// upgrade neither falls back to a raw TLS handshake nor leaks the message over
// plaintext when the server does not advertise the capability.
func TestSendEmailStartTLSRequiredFailsClosed(t *testing.T) {
	ln, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		t.Fatalf("Listen() error = %v", err)
	}
	defer ln.Close()

	server := &fakeSMTPServer{}
	done := make(chan struct{})
	go func() {
		defer close(done)
		server.serveOne(ln, "250-fake.example.com\r\n250 AUTH LOGIN\r\n")
	}()

	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	err = sendEmailStartTLS(ctx, "localhost", ln.Addr().String(), &tls.Config{ServerName: "localhost", MinVersion: tls.VersionTLS12}, "", "", "from@example.com", []string{"to@example.com"}, []byte("test"), emailTransportRequiredSTARTTLS)
	if err == nil {
		t.Fatal("sendEmailStartTLS() error = nil, want STARTTLS-not-supported error")
	}
	if strings.Contains(err.Error(), "tls: first record does not look like a TLS handshake") {
		t.Fatalf("sendEmailStartTLS() attempted a raw TLS handshake: %v", err)
	}

	<-done
	for _, command := range server.received() {
		if strings.HasPrefix(strings.ToUpper(command), "MAIL FROM") {
			t.Fatalf("sendEmailStartTLS() sent %q over plaintext instead of failing closed", command)
		}
	}
}

// TestSendEmailStartTLSAbortsOnContextTimeout guards against SMTP operations
// blocking past the caller's context deadline once the TCP connection has
// already been established (e.g. an unresponsive/slow SMTP peer).
func TestSendEmailStartTLSAbortsOnContextTimeout(t *testing.T) {
	ln, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		t.Fatalf("Listen() error = %v", err)
	}
	defer ln.Close()

	accepted := make(chan struct{})
	go func() {
		conn, err := ln.Accept()
		if err != nil {
			return
		}
		defer conn.Close()
		close(accepted)
		// Simulate an unresponsive SMTP server that never sends the greeting banner.
		time.Sleep(500 * time.Millisecond)
	}()

	ctx, cancel := context.WithTimeout(context.Background(), 50*time.Millisecond)
	defer cancel()

	start := time.Now()
	err = sendEmailStartTLS(ctx, "localhost", ln.Addr().String(), &tls.Config{ServerName: "localhost", MinVersion: tls.VersionTLS12}, "", "", "from@example.com", []string{"to@example.com"}, []byte("test"), emailTransportOpportunisticSTARTTLS)
	elapsed := time.Since(start)

	<-accepted
	if err == nil {
		t.Fatal("sendEmailStartTLS() error = nil, want error once context times out")
	}
	if elapsed > 300*time.Millisecond {
		t.Fatalf("sendEmailStartTLS() took %v, want it to return shortly after the 50ms context timeout instead of waiting for the peer", elapsed)
	}
}

// TestSendEmailImplicitTLSDeliversMessage covers backward compatibility with
// SMTPS servers that expect a TLS handshake before the SMTP greeting.
func TestSendEmailImplicitTLSDeliversMessage(t *testing.T) {
	cert, pool := newTestTLSCertificate(t)
	ln, err := tls.Listen("tcp", "127.0.0.1:0", &tls.Config{
		Certificates: []tls.Certificate{cert},
		MinVersion:   tls.VersionTLS12,
	})
	if err != nil {
		t.Fatalf("tls.Listen() error = %v", err)
	}
	defer ln.Close()

	server := &fakeSMTPServer{}
	done := make(chan struct{})
	go func() {
		defer close(done)
		server.serveOne(ln, "250-fake.example.com\r\n250 SIZE 10240000\r\n")
	}()

	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	err = sendEmailImplicitTLS(
		ctx,
		"localhost",
		localhostAddr(t, ln.Addr().String()),
		&tls.Config{ServerName: "localhost", MinVersion: tls.VersionTLS12, RootCAs: pool},
		"", "",
		"from@example.com",
		[]string{"to@example.com"},
		[]byte("Subject: test\r\n\r\nbody"),
	)
	if err != nil {
		t.Fatalf("sendEmailImplicitTLS() error = %v", err)
	}

	<-done
	commands := strings.ToUpper(strings.Join(server.received(), "\n"))
	for _, want := range []string{"MAIL FROM", "RCPT TO", "DATA", "QUIT"} {
		if !strings.Contains(commands, want) {
			t.Fatalf("server received %q, want it to contain %q", commands, want)
		}
	}
}

// TestSendEmailLegacySSLTrueUsesImplicitTLS pins the backward-compatible
// mapping: an existing config with SSL=true and no explicit encryption mode
// must still perform implicit TLS rather than waiting for a plaintext greeting.
func TestSendEmailLegacySSLTrueUsesImplicitTLS(t *testing.T) {
	transport, err := parseEmailTransport("", true)
	if err != nil {
		t.Fatalf("parseEmailTransport() error = %v", err)
	}

	ln, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		t.Fatalf("Listen() error = %v", err)
	}
	defer ln.Close()

	server := &fakeSMTPServer{}
	go server.serveOne(ln, "250 fake.example.com\r\n")

	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	err = sendEmail(ctx, "localhost", ln.Addr().String(), "", "", "from@example.com", []string{"to@example.com"}, []byte("test"), transport)
	if err == nil {
		t.Fatal("sendEmail() error = nil, want TLS handshake failure against a plaintext server")
	}
	// A plaintext greeting cannot satisfy a TLS ClientHello, which proves the
	// implicit-TLS path was taken instead of STARTTLS.
	if !strings.Contains(err.Error(), "first record does not look like a TLS handshake") {
		t.Fatalf("sendEmail() error = %v, want an implicit TLS handshake failure", err)
	}
}

func TestSendEmailImplicitTLSAbortsOnContextTimeout(t *testing.T) {
	ln, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		t.Fatalf("Listen() error = %v", err)
	}
	defer ln.Close()

	accepted := make(chan struct{})
	go func() {
		conn, err := ln.Accept()
		if err != nil {
			return
		}
		defer conn.Close()
		close(accepted)
		// Never complete the TLS handshake.
		time.Sleep(500 * time.Millisecond)
	}()

	ctx, cancel := context.WithTimeout(context.Background(), 50*time.Millisecond)
	defer cancel()

	start := time.Now()
	err = sendEmailImplicitTLS(ctx, "localhost", ln.Addr().String(), &tls.Config{ServerName: "localhost", MinVersion: tls.VersionTLS12}, "", "", "from@example.com", []string{"to@example.com"}, []byte("test"))
	elapsed := time.Since(start)

	<-accepted
	if err == nil {
		t.Fatal("sendEmailImplicitTLS() error = nil, want error once context times out")
	}
	if elapsed > 300*time.Millisecond {
		t.Fatalf("sendEmailImplicitTLS() took %v, want it to return shortly after the 50ms context timeout", elapsed)
	}
}

// fakeSMTPServer serves a single SMTP conversation and records the commands it
// received, so tests can assert which commands were (not) sent.
type fakeSMTPServer struct {
	mu       sync.Mutex
	commands []string
}

func (s *fakeSMTPServer) record(command string) {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.commands = append(s.commands, command)
}

func (s *fakeSMTPServer) received() []string {
	s.mu.Lock()
	defer s.mu.Unlock()
	return append([]string(nil), s.commands...)
}

func (s *fakeSMTPServer) serveOne(ln net.Listener, ehloResponse string) {
	conn, err := ln.Accept()
	if err != nil {
		return
	}
	defer conn.Close()
	_ = conn.SetDeadline(time.Now().Add(5 * time.Second))

	if _, err := conn.Write([]byte("220 fake.example.com ESMTP\r\n")); err != nil {
		return
	}

	reader := bufio.NewReader(conn)
	for {
		line, err := reader.ReadString('\n')
		if err != nil {
			return
		}

		command := strings.TrimSpace(line)
		s.record(command)

		switch upper := strings.ToUpper(command); {
		case strings.HasPrefix(upper, "EHLO"), strings.HasPrefix(upper, "HELO"):
			_, _ = conn.Write([]byte(ehloResponse))
		case strings.HasPrefix(upper, "MAIL FROM"), strings.HasPrefix(upper, "RCPT TO"):
			_, _ = conn.Write([]byte("250 OK\r\n"))
		case strings.HasPrefix(upper, "DATA"):
			_, _ = conn.Write([]byte("354 End data with <CR><LF>.<CR><LF>\r\n"))
			for {
				bodyLine, err := reader.ReadString('\n')
				if err != nil {
					return
				}
				if strings.TrimSpace(bodyLine) == "." {
					break
				}
			}
			_, _ = conn.Write([]byte("250 OK\r\n"))
		case strings.HasPrefix(upper, "QUIT"):
			_, _ = conn.Write([]byte("221 Bye\r\n"))
			return
		default:
			_, _ = conn.Write([]byte("500 unrecognized command\r\n"))
		}
	}
}

// newTestTLSCertificate returns a short-lived self-signed certificate for
// localhost together with a pool that trusts it.
func newTestTLSCertificate(t *testing.T) (tls.Certificate, *x509.CertPool) {
	t.Helper()

	key, err := ecdsa.GenerateKey(elliptic.P256(), rand.Reader)
	if err != nil {
		t.Fatalf("GenerateKey() error = %v", err)
	}

	template := x509.Certificate{
		SerialNumber:          big.NewInt(1),
		Subject:               pkix.Name{CommonName: "localhost"},
		NotBefore:             time.Now().Add(-time.Hour),
		NotAfter:              time.Now().Add(time.Hour),
		KeyUsage:              x509.KeyUsageDigitalSignature | x509.KeyUsageCertSign,
		ExtKeyUsage:           []x509.ExtKeyUsage{x509.ExtKeyUsageServerAuth},
		DNSNames:              []string{"localhost"},
		IPAddresses:           []net.IP{net.ParseIP("127.0.0.1")},
		BasicConstraintsValid: true,
		IsCA:                  true,
	}

	der, err := x509.CreateCertificate(rand.Reader, &template, &template, &key.PublicKey, key)
	if err != nil {
		t.Fatalf("CreateCertificate() error = %v", err)
	}

	leaf, err := x509.ParseCertificate(der)
	if err != nil {
		t.Fatalf("ParseCertificate() error = %v", err)
	}

	pool := x509.NewCertPool()
	pool.AddCert(leaf)

	return tls.Certificate{Certificate: [][]byte{der}, PrivateKey: key, Leaf: leaf}, pool
}

// localhostAddr rewrites a listener address to use the "localhost" hostname so
// it matches the test certificate's SAN.
func localhostAddr(t *testing.T, addr string) string {
	t.Helper()

	_, port, err := net.SplitHostPort(addr)
	if err != nil {
		t.Fatalf("SplitHostPort() error = %v", err)
	}

	return net.JoinHostPort("localhost", port)
}

func TestLoginAuthRejectsUnencryptedNonLocalhostConnection(t *testing.T) {
	auth := &loginAuth{username: "user", password: "secret"}
	if _, _, err := auth.Start(&smtp.ServerInfo{Name: "mail.example.com", TLS: false}); err == nil {
		t.Fatal("Start() error = nil, want error for unencrypted non-localhost connection")
	}
}

func TestLoginAuthAllowsLocalhostOrTLSConnection(t *testing.T) {
	auth := &loginAuth{username: "user", password: "secret"}
	if _, _, err := auth.Start(&smtp.ServerInfo{Name: "localhost", TLS: false}); err != nil {
		t.Fatalf("Start() error = %v, want nil for localhost", err)
	}
	if _, _, err := auth.Start(&smtp.ServerInfo{Name: "mail.example.com", TLS: true}); err != nil {
		t.Fatalf("Start() error = %v, want nil for TLS connection", err)
	}
}

func TestLoginAuthRespondsWithUsernameAndPassword(t *testing.T) {
	auth := &loginAuth{username: "user", password: "secret"}
	mechanism, initialResponse, err := auth.Start(&smtp.ServerInfo{Name: "localhost", TLS: false})
	if err != nil {
		t.Fatalf("Start() error = %v", err)
	}
	if mechanism != "LOGIN" || initialResponse != nil {
		t.Fatalf("Start() = (%q, %q), want (LOGIN, nil)", mechanism, initialResponse)
	}

	username, err := auth.Next(nil, true)
	if err != nil || string(username) != "user" {
		t.Fatalf("first Next() = (%q, %v), want (user, nil)", username, err)
	}
	password, err := auth.Next(nil, true)
	if err != nil || string(password) != "secret" {
		t.Fatalf("second Next() = (%q, %v), want (secret, nil)", password, err)
	}
}

func TestSupportsSMTPAuth(t *testing.T) {
	if !supportsSMTPAuth("PLAIN LOGIN", "LOGIN") {
		t.Fatal("supportsSMTPAuth() = false, want true")
	}
	if supportsSMTPAuth("PLAIN LOGIN", "CRAM-MD5") {
		t.Fatal("supportsSMTPAuth() = true, want false")
	}
}

func TestBuildEmailMessageUsesPlainTextByDefault(t *testing.T) {
	message, err := buildEmailMessage("sender@example.com", "receiver@example.com", "Plain subject", "Line 1\nLine 2", false, "")
	if err != nil {
		t.Fatalf("buildEmailMessage() error = %v", err)
	}

	body := string(message)
	if !strings.Contains(body, "Content-Type: text/plain; charset=UTF-8") {
		t.Fatalf("message content type = %q, want text/plain", body)
	}
	if !strings.Contains(body, "Plain subject\n\nLine 1\nLine 2") {
		t.Fatalf("message body = %q, want plain title and content", body)
	}
}

func TestBuildEmailMessageUsesHTMLTemplate(t *testing.T) {
	message, err := buildEmailMessage("sender@example.com", "receiver@example.com", "HTML subject", "<b>Line 1</b>\nLine 2", true, "")
	if err != nil {
		t.Fatalf("buildEmailMessage() error = %v", err)
	}

	body := string(message)
	if !strings.Contains(body, "Content-Type: text/html; charset=UTF-8") {
		t.Fatalf("message content type = %q, want text/html", body)
	}
	if !strings.Contains(body, "<h1 style=\"margin: 0 0 16px; color: #1677ff;\">HTML subject</h1>") {
		t.Fatalf("message body = %q, want HTML title", body)
	}
	if !strings.Contains(body, "&lt;b&gt;Line 1&lt;/b&gt;<br>\nLine 2") {
		t.Fatalf("message body = %q, want escaped HTML content with line breaks", body)
	}
}

func TestBuildEmailMessageUsesCustomHTMLTemplate(t *testing.T) {
	message, err := buildEmailMessage(
		"sender@example.com",
		"receiver@example.com",
		"Custom subject",
		"Line 1\nLine 2",
		true,
		`<main><h1>{{.Title}}</h1><article>{{.Content}}</article></main>`,
	)
	if err != nil {
		t.Fatalf("buildEmailMessage() error = %v", err)
	}

	body := string(message)
	if !strings.Contains(body, "<main><h1>Custom subject</h1><article>Line 1<br>\nLine 2</article></main>") {
		t.Fatalf("message body = %q, want custom HTML template", body)
	}
}
