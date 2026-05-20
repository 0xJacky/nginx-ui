package ssh

import (
	"bytes"
	"context"
	"errors"
	"fmt"
	"net"
	"os"
	"sync"
	"time"

	"github.com/uozi-tech/cosy"
	"github.com/uozi-tech/cosy/logger"
	gossh "golang.org/x/crypto/ssh"
)

// ClientOptions holds everything Client.Dial needs to bring up a session.
type ClientOptions struct {
	Address        string // host:port
	User           string
	AuthMethod     string // "key" | "password"
	PrivateKeyPath string
	Password       string // decrypted password, only when AuthMethod=="password"
	KnownHosts     *KnownHosts
	Strict         bool          // if false, accept any host key on first connect
	Timeout        time.Duration // dial+handshake timeout; default 10s
	KeepAlive      time.Duration // SSH-level keepalive; default 30s
	Config         Config        // forwarded into Exec
}

// Client maintains a single long-lived SSH connection that all Exec calls share.
type Client struct {
	opts ClientOptions
	mu   sync.Mutex
	conn *gossh.Client
}

func NewClient(opts ClientOptions) *Client {
	if opts.Timeout == 0 {
		opts.Timeout = 10 * time.Second
	}
	if opts.KeepAlive == 0 {
		opts.KeepAlive = 30 * time.Second
	}
	return &Client{opts: opts}
}

func (c *Client) dial(ctx context.Context) (*gossh.Client, error) {
	authMethods, err := c.buildAuth()
	if err != nil {
		return nil, err
	}

	hostKeyCallback := gossh.InsecureIgnoreHostKey()
	if c.opts.KnownHosts != nil {
		hostKeyCallback = c.opts.KnownHosts.HostKeyCallback()
	}
	if c.opts.Strict && c.opts.KnownHosts == nil {
		return nil, errors.New("strict host key checking enabled but no known_hosts configured")
	}

	cfg := &gossh.ClientConfig{
		User:            c.opts.User,
		Auth:            authMethods,
		HostKeyCallback: hostKeyCallback,
		Timeout:         c.opts.Timeout,
	}

	dialer := net.Dialer{Timeout: c.opts.Timeout}
	tcp, err := dialer.DialContext(ctx, "tcp", c.opts.Address)
	if err != nil {
		return nil, cosy.WrapErrorWithParams(ErrConnectFailed, err.Error())
	}
	sshConn, chans, reqs, err := gossh.NewClientConn(tcp, c.opts.Address, cfg)
	if err != nil {
		_ = tcp.Close()
		return nil, cosy.WrapErrorWithParams(ErrAuthFailed, err.Error())
	}
	client := gossh.NewClient(sshConn, chans, reqs)
	go c.keepalive(client)
	return client, nil
}

func (c *Client) buildAuth() ([]gossh.AuthMethod, error) {
	switch c.opts.AuthMethod {
	case "password":
		return []gossh.AuthMethod{gossh.Password(c.opts.Password)}, nil
	default: // "key" (or empty)
		raw, err := os.ReadFile(c.opts.PrivateKeyPath)
		if err != nil {
			return nil, cosy.WrapErrorWithParams(ErrAuthFailed, err.Error())
		}
		signer, err := gossh.ParsePrivateKey(raw)
		if err != nil {
			return nil, cosy.WrapErrorWithParams(ErrAuthFailed, err.Error())
		}
		return []gossh.AuthMethod{gossh.PublicKeys(signer)}, nil
	}
}

func (c *Client) keepalive(client *gossh.Client) {
	t := time.NewTicker(c.opts.KeepAlive)
	defer t.Stop()
	for range t.C {
		_, _, err := client.SendRequest("keepalive@nginx-ui", true, nil)
		if err != nil {
			logger.Warn("ssh keepalive failed; client will reconnect on next Exec", "err", err)
			return
		}
	}
}

// connect returns a healthy client, reconnecting if the cached one is dead.
func (c *Client) connect(ctx context.Context) (*gossh.Client, error) {
	c.mu.Lock()
	if c.conn != nil {
		if _, _, err := c.conn.SendRequest("keepalive@nginx-ui", true, nil); err == nil {
			defer c.mu.Unlock()
			return c.conn, nil
		}
		_ = c.conn.Close()
		c.conn = nil
	}
	c.mu.Unlock()

	conn, err := c.dial(ctx)
	if err != nil {
		return nil, err
	}
	c.mu.Lock()
	c.conn = conn
	c.mu.Unlock()
	return conn, nil
}

// Exec runs a single command and returns combined stdout/stderr.
func (c *Client) Exec(ctx context.Context, name string, args ...string) (string, error) {
	conn, err := c.connect(ctx)
	if err != nil {
		return "", err
	}
	sess, err := conn.NewSession()
	if err != nil {
		return "", cosy.WrapErrorWithParams(ErrSessionFailed, err.Error())
	}
	defer sess.Close()

	var out bytes.Buffer
	sess.Stdout = &out
	sess.Stderr = &out

	cmd := buildCommand(c.opts.Config, name, args)
	done := make(chan error, 1)
	go func() { done <- sess.Run(cmd) }()

	select {
	case err := <-done:
		if err != nil {
			return out.String(), fmt.Errorf("ssh exec %q: %w (stderr: %s)", cmd, err, out.String())
		}
		return out.String(), nil
	case <-ctx.Done():
		_ = sess.Signal(gossh.SIGTERM)
		return out.String(), cosy.WrapErrorWithParams(ErrCommandTimeout, ctx.Err().Error())
	}
}

// Stat checks remote file existence via a tiny `test -e` invocation.
func (c *Client) Stat(path string) bool {
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	_, err := c.Exec(ctx, "/usr/bin/test", "-e", path)
	return err == nil
}

// Close releases the cached connection if any.
func (c *Client) Close() error {
	c.mu.Lock()
	defer c.mu.Unlock()
	if c.conn != nil {
		err := c.conn.Close()
		c.conn = nil
		return err
	}
	return nil
}
