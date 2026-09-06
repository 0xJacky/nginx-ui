//go:build !windows

package process

import (
	"bufio"
	"context"
	"errors"
	"io"
	"net"
	"net/http"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"code.pfad.fr/risefront"
	"github.com/pires/go-proxyproto"
)

func TestListenerHandover(t *testing.T) {
	for _, network := range []string{"tcp", "unix"} {
		t.Run(network, func(t *testing.T) {
			oldPolicy := proxyproto.DefaultPolicy
			ConfigureProxyProtocol(network)
			if network == "tcp" && proxyproto.DefaultPolicy != oldPolicy {
				t.Fatal("TCP proxy policy changed")
			}
			defer func() { proxyproto.DefaultPolicy = oldPolicy }()
			// Keep paths short enough for the Unix socket limit on macOS.
			dir, err := os.MkdirTemp("/tmp", "nui-")
			if err != nil {
				t.Fatal(err)
			}
			defer os.RemoveAll(dir)
			address := "127.0.0.1:0"
			if network == "unix" {
				address = filepath.Join(dir, "http.sock")
			}
			ctx, cancel := context.WithCancel(context.Background())
			defer cancel()
			release := make(chan struct{})
			defer func() {
				select {
				case <-release:
				default:
					close(release)
				}
			}()
			started := make(chan struct{})
			type instance struct {
				ready   chan net.Addr
				retired chan struct{}
				done    chan error
			}
			start := func(body string) instance {
				inst := instance{make(chan net.Addr, 1), make(chan struct{}), make(chan error, 1)}
				go func() {
					inst.done <- risefront.New(ctx, risefront.Config{
						Name: "test", Network: network, Addresses: []string{address},
						Dialer: risefront.PrefixDialer{WorkingDirectory: dir}, NoRestart: true,
						LogHandler: func(risefront.LogLevel, string, ...any) {},
						Run: func(listeners []net.Listener) error {
							listener := NewLifecycleListener(listeners[0])
							server := &http.Server{Handler: http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
								if r.URL.Path == "/slow" {
									close(started)
									<-release
								}
								if r.URL.Path == "/remote" {
									_, _ = io.WriteString(w, r.RemoteAddr)
									return
								}
								_, _ = io.WriteString(w, body)
							})}
							inst.ready <- listener.Addr()
							err := server.Serve(listener)
							<-listener.Done()
							close(inst.retired)
							shutdownCtx, stop := context.WithTimeout(context.Background(), 5*time.Second)
							defer stop()
							if shutdownErr := server.Shutdown(shutdownCtx); shutdownErr != nil {
								return shutdownErr
							}
							if errors.Is(err, http.ErrServerClosed) || errors.Is(err, net.ErrClosed) {
								return nil
							}
							return err
						},
					})
				}()
				return inst
			}
			wait := func(ch <-chan struct{}) {
				t.Helper()
				select {
				case <-ch:
				case <-time.After(10 * time.Second):
					t.Fatal("listener lifecycle timed out")
				}
			}
			ready := func(inst instance) net.Addr {
				t.Helper()
				select {
				case addr := <-inst.ready:
					return addr
				case err := <-inst.done:
					t.Fatalf("listener failed: %v", err)
				case <-time.After(10 * time.Second):
					t.Fatal("listener startup timed out")
				}
				return nil
			}
			parent := start("parent")
			external := ready(parent).String()
			transport := &http.Transport{DisableKeepAlives: true, DialContext: func(ctx context.Context, _, _ string) (net.Conn, error) {
				return (&net.Dialer{}).DialContext(ctx, network, external)
			}}
			defer transport.CloseIdleConnections()
			client := &http.Client{Transport: transport, Timeout: 10 * time.Second}
			request := func(path string) (string, error) {
				resp, err := client.Get("http://localhost" + path)
				if err != nil {
					return "", err
				}
				defer resp.Body.Close()
				data, err := io.ReadAll(resp.Body)
				return string(data), err
			}
			check := func(want string) {
				t.Helper()
				got, err := request("/")
				if err != nil || got != want {
					t.Fatalf("HTTP response = %q, %v; want %q", got, err, want)
				}
			}
			check("parent")
			slow := make(chan string, 1)
			go func() {
				body, err := request("/slow")
				if err != nil {
					body = err.Error()
				}
				slow <- body
			}()
			wait(started)
			child := start("child")
			ready(child)
			wait(parent.retired)
			check("child")
			if network == "unix" {
				if _, err := os.Stat(address); err != nil {
					t.Fatalf("handover removed socket: %v", err)
				}
				// A client-supplied PROXY header must not become a trusted address
				// when risefront forwards the raw Unix connection to the child.
				conn, err := net.DialTimeout("unix", address, time.Second)
				if err != nil {
					t.Fatal(err)
				}
				defer conn.Close()
				if err := conn.SetDeadline(time.Now().Add(5 * time.Second)); err != nil {
					t.Fatal(err)
				}
				_, err = io.WriteString(conn, "PROXY TCP4 203.0.113.10 203.0.113.20 1234 80\r\nGET /remote HTTP/1.1\r\nHost: localhost\r\nConnection: close\r\n\r\n")
				if err != nil {
					t.Fatal(err)
				}
				resp, err := http.ReadResponse(bufio.NewReader(conn), nil)
				if err != nil {
					t.Fatal(err)
				}
				data, err := io.ReadAll(resp.Body)
				resp.Body.Close()
				conn.Close()
				if err != nil || resp.StatusCode != http.StatusOK || strings.Contains(string(data), "203.0.113.") {
					t.Fatalf("client address was not preserved: %q, status %d, %v", data, resp.StatusCode, err)
				}
			}
			close(release)
			select {
			case got := <-slow:
				if got != "parent" {
					t.Fatalf("in-flight response = %q", got)
				}
			case <-time.After(10 * time.Second):
				t.Fatal("in-flight request did not drain")
			}
			cancel()
			for _, inst := range []instance{parent, child} {
				select {
				case err := <-inst.done:
					if err != nil && !errors.Is(err, context.Canceled) {
						t.Fatal(err)
					}
				case <-time.After(10 * time.Second):
					t.Fatal("shutdown timed out")
				}
				wait(inst.retired)
			}
			entries, err := os.ReadDir(dir)
			if err != nil {
				t.Fatal(err)
			}
			if len(entries) != 0 {
				t.Fatalf("socket files remain after shutdown: %v", entries)
			}
		})
	}
}

func TestUnixListenerPreservesExistingPath(t *testing.T) {
	for _, activeSocket := range []bool{false, true} {
		dir, err := os.MkdirTemp("/tmp", "nui-")
		if err != nil {
			t.Fatal(err)
		}
		t.Cleanup(func() { os.RemoveAll(dir) })
		socketPath := filepath.Join(dir, "http.sock")
		if activeSocket {
			listener, err := net.Listen("unix", socketPath)
			if err != nil {
				t.Fatal(err)
			}
			defer listener.Close()
		} else if err := os.WriteFile(socketPath, []byte("keep"), 0600); err != nil {
			t.Fatal(err)
		}
		before, err := os.Lstat(socketPath)
		if err != nil {
			t.Fatal(err)
		}
		ctx, cancel := context.WithCancel(context.Background())
		err = risefront.New(ctx, risefront.Config{
			Network: "unix", Addresses: []string{socketPath}, Name: "test",
			Dialer: risefront.PrefixDialer{WorkingDirectory: dir}, NoRestart: true,
			Run:        func([]net.Listener) error { t.Error("unexpected HTTP startup"); return nil },
			LogHandler: func(risefront.LogLevel, string, ...any) {},
		})
		cancel()
		if err == nil {
			t.Fatal("expected address-in-use error")
		}
		after, err := os.Lstat(socketPath)
		if err != nil || !os.SameFile(before, after) {
			t.Fatalf("existing path was replaced: %v", err)
		}
		if _, err := os.Stat(filepath.Join(dir, "test.sock")); !os.IsNotExist(err) {
			t.Fatalf("control socket not cleaned up: %v", err)
		}
	}
}
