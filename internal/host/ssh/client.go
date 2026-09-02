package ssh

import (
	"bytes"
	"context"
	"errors"
	"fmt"
	"net"
	"os"
	"sync"
	"sync/atomic"
	"time"

	"github.com/pkg/sftp"
	"github.com/uozi-tech/cosy"
	"github.com/uozi-tech/cosy/logger"
	gossh "golang.org/x/crypto/ssh"
)

// ClientOptions holds everything Client.Dial needs to bring up a session.
type ClientOptions struct {
	Address        string // host:port
	User           string
	PrivateKeyPath string
	KnownHosts     *KnownHosts
	Timeout        time.Duration // dial+handshake timeout; default 10s
	KeepAlive      time.Duration // SSH-level keepalive; default 30s
	Config         Config        // forwarded into Exec
}

// Client maintains a single long-lived SSH connection that all Exec calls share.
type Client struct {
	opts ClientOptions
	mu   sync.Mutex
	conn *gossh.Client
	sftp *sftp.Client
	// closed is terminal. Without it a discarded client would silently redial
	// the old host with the options captured when it was built.
	closed bool
	// lastProbe is the wall-clock time (UnixNano) of the last request the
	// cached connection answered. connectLocked only sends a synchronous probe
	// when this is older than the keepalive interval, so a healthy connection
	// that the keepalive goroutine is already exercising costs Exec and SFTP
	// callers no extra round trip.
	lastProbe atomic.Int64
}

// synchronizedBuffer is safe for SSH's concurrent stdout and extended-data
// copy goroutines. A plain bytes.Buffer can lose stderr when both streams use it.
type synchronizedBuffer struct {
	mu  sync.Mutex
	buf bytes.Buffer
}

func (b *synchronizedBuffer) Write(p []byte) (int, error) {
	b.mu.Lock()
	defer b.mu.Unlock()
	return b.buf.Write(p)
}

func (b *synchronizedBuffer) String() string {
	b.mu.Lock()
	defer b.mu.Unlock()
	return b.buf.String()
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

	hostKeyCallback, err := c.hostKeyCallback()
	if err != nil {
		return nil, err
	}
	hostKeyAlgorithms, err := c.opts.KnownHosts.HostKeyAlgorithms(c.opts.Address)
	if err != nil {
		return nil, err
	}

	cfg := &gossh.ClientConfig{
		User:              c.opts.User,
		Auth:              authMethods,
		HostKeyCallback:   hostKeyCallback,
		HostKeyAlgorithms: hostKeyAlgorithms,
		Timeout:           c.opts.Timeout,
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
	c.markProbed()
	go c.keepalive(client)
	return client, nil
}

// markProbed records that the cached connection just answered a request.
func (c *Client) markProbed() {
	c.lastProbe.Store(time.Now().UnixNano())
}

// needsProbe reports whether the cached connection has gone unverified for
// longer than the keepalive interval and must be probed before reuse.
func (c *Client) needsProbe(now time.Time) bool {
	last := c.lastProbe.Load()
	if last == 0 {
		return true
	}
	return now.Sub(time.Unix(0, last)) >= c.opts.KeepAlive
}

func (c *Client) hostKeyCallback() (gossh.HostKeyCallback, error) {
	if c.opts.KnownHosts == nil {
		return nil, errors.New("known_hosts allow-list is required for ssh host key verification")
	}
	callback := c.opts.KnownHosts.HostKeyCallback()
	if callback == nil {
		return nil, errors.New("known_hosts host key callback is unavailable")
	}
	return callback, nil
}

// MaxPrivateKeyFileSize bounds every private key read. The path is operator
// supplied, so reading a character device such as /dev/zero would otherwise
// never end.
const MaxPrivateKeyFileSize = 64 * 1024

// ReadPrivateKeyFile reads a private key from disk, rejecting anything that is
// not a regular file of a plausible size.
func ReadPrivateKeyFile(path string) ([]byte, error) {
	info, err := os.Stat(path)
	if err != nil {
		return nil, err
	}
	if !info.Mode().IsRegular() {
		return nil, fmt.Errorf("private key %s is not a regular file", path)
	}
	if info.Size() > MaxPrivateKeyFileSize {
		return nil, fmt.Errorf("private key %s is too large", path)
	}
	return os.ReadFile(path)
}

func (c *Client) buildAuth() ([]gossh.AuthMethod, error) {
	raw, err := ReadPrivateKeyFile(c.opts.PrivateKeyPath)
	if err != nil {
		return nil, cosy.WrapErrorWithParams(ErrAuthFailed, err.Error())
	}
	signer, err := gossh.ParsePrivateKey(raw)
	if err != nil {
		return nil, cosy.WrapErrorWithParams(ErrAuthFailed, err.Error())
	}
	return []gossh.AuthMethod{gossh.PublicKeys(signer)}, nil
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
		c.markProbed()
	}
}

// connectLocked returns a healthy client, reconnecting if the cached one is
// dead. The caller holds c.mu across dial so concurrent callers serialize on
// reconnect rather than racing to overwrite c.conn.
func (c *Client) connectLocked(ctx context.Context) (*gossh.Client, error) {
	if c.closed {
		return nil, ErrClientClosed
	}

	if c.conn != nil {
		if !c.needsProbe(time.Now()) {
			return c.conn, nil
		}
		if alive(ctx, c.conn, c.opts.KeepAlive) {
			c.markProbed()
			return c.conn, nil
		}
		c.dropLocked()
	}

	conn, err := c.dial(ctx)
	if err != nil {
		return nil, err
	}
	c.conn = conn
	return conn, nil
}

// dropLocked discards the cached connection and its SFTP session.
func (c *Client) dropLocked() {
	if c.sftp != nil {
		_ = c.sftp.Close()
		c.sftp = nil
	}
	if c.conn != nil {
		_ = c.conn.Close()
		c.conn = nil
	}
}

// redialLocked replaces stale, a connection that just failed to open a
// session, with a fresh one. A connection another caller has already
// replaced is left alone so the newer one is not torn down by mistake.
func (c *Client) redialLocked(ctx context.Context, stale *gossh.Client) (*gossh.Client, error) {
	if c.conn == stale {
		c.dropLocked()
	}
	return c.connectLocked(ctx)
}

func (c *Client) connect(ctx context.Context) (*gossh.Client, error) {
	c.mu.Lock()
	defer c.mu.Unlock()
	return c.connectLocked(ctx)
}

func (c *Client) redial(ctx context.Context, stale *gossh.Client) (*gossh.Client, error) {
	c.mu.Lock()
	defer c.mu.Unlock()
	return c.redialLocked(ctx, stale)
}

// SFTP returns an SFTP session bound to the current verified SSH connection.
// When the SSH connection is replaced, the stale SFTP session is closed before
// a new one is created.
func (c *Client) SFTP(ctx context.Context) (*sftp.Client, error) {
	c.mu.Lock()
	defer c.mu.Unlock()

	conn, err := c.connectLocked(ctx)
	if err != nil {
		return nil, err
	}
	if c.sftp != nil {
		return c.sftp, nil
	}

	client, err := sftp.NewClient(conn)
	if err != nil {
		// The connection may have died since the last probe; one redial
		// keeps the recovery transparent without looping on a bad host.
		conn, err = c.redialLocked(ctx, conn)
		if err != nil {
			return nil, err
		}
		client, err = sftp.NewClient(conn)
		if err != nil {
			return nil, fmt.Errorf("start sftp subsystem: %w", err)
		}
	}
	c.sftp = client
	return client, nil
}

// Exec runs a single command and returns combined stdout/stderr.
func (c *Client) Exec(ctx context.Context, name string, args ...string) (string, error) {
	conn, err := c.connect(ctx)
	if err != nil {
		return "", err
	}
	sess, err := conn.NewSession()
	if err != nil {
		// The connection may have died since the last probe; one redial
		// keeps the recovery transparent without looping on a bad host.
		conn, err = c.redial(ctx, conn)
		if err != nil {
			return "", err
		}
		sess, err = conn.NewSession()
		if err != nil {
			return "", cosy.WrapErrorWithParams(ErrSessionFailed, err.Error())
		}
	}
	defer sess.Close()

	var out synchronizedBuffer
	sess.Stdout = &out
	sess.Stderr = &out

	cmd := buildCommand(c.opts.Config, name, args)
	done := make(chan error, 1)
	go func() { done <- sess.Run(cmd) }()

	select {
	case err := <-done:
		// A completed session proves the connection is good even when the
		// command itself exited non-zero; any other failure may be the link.
		var exitErr *gossh.ExitError
		if err == nil || errors.As(err, &exitErr) {
			c.markProbed()
		}
		if err != nil {
			return out.String(), fmt.Errorf("ssh exec %q: %w (stderr: %s)", cmd, err, out.String())
		}
		return out.String(), nil
	case <-ctx.Done():
		_ = sess.Signal(gossh.SIGTERM)
		return out.String(), cosy.WrapErrorWithParams(ErrCommandTimeout, ctx.Err().Error())
	}
}

// remoteTestBinary is the absolute path of test(1) on the SSH host. Both Linux
// and macOS ship it at /bin/test, whereas /usr/bin/test only exists on Linux.
const remoteTestBinary = "/bin/test"

// statCommand builds the probe Stat runs on the host.
func statCommand(path string) (string, []string) {
	return remoteTestBinary, []string{"-e", path}
}

// Stat checks remote file existence via a tiny `test -e` invocation.
func (c *Client) Stat(path string) bool {
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	name, args := statCommand(path)
	_, err := c.Exec(ctx, name, args...)
	return err == nil
}

// alive probes the cached connection without blocking the mutex on a
// half-open socket, where SendRequest waits for the TCP timeout.
func alive(ctx context.Context, conn *gossh.Client, timeout time.Duration) bool {
	if timeout <= 0 {
		timeout = 5 * time.Second
	}
	done := make(chan error, 1)
	go func() {
		_, _, err := conn.SendRequest("keepalive@nginx-ui", true, nil)
		done <- err
	}()

	timer := time.NewTimer(timeout)
	defer timer.Stop()
	select {
	case err := <-done:
		return err == nil
	case <-timer.C:
		return false
	case <-ctx.Done():
		return false
	}
}

// Close releases the cached connection and marks the client unusable, so a
// caller holding a stale reference fails loudly instead of redialing the host
// this client was built for.
func (c *Client) Close() error {
	c.mu.Lock()
	defer c.mu.Unlock()
	c.closed = true
	if c.sftp != nil {
		_ = c.sftp.Close()
		c.sftp = nil
	}
	if c.conn != nil {
		err := c.conn.Close()
		c.conn = nil
		return err
	}
	return nil
}
