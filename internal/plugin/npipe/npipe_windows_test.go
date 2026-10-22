//go:build windows

package npipe

import (
	"context"
	"io"
	"testing"
	"time"

	"github.com/Microsoft/go-winio"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestDial(t *testing.T) {
	name := Prefix + "nginx-ui-npipe-test-" + time.Now().Format("150405.000000000")
	l, err := winio.ListenPipe(name, nil)
	require.NoError(t, err)
	defer l.Close()
	go func() {
		conn, err := l.Accept()
		if err == nil {
			_, _ = conn.Write([]byte("hello"))
			_ = conn.Close()
		}
	}()

	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	conn, err := Dial(ctx, name)
	require.NoError(t, err)
	defer conn.Close()
	got, err := io.ReadAll(conn)
	require.NoError(t, err)
	assert.Equal(t, "hello", string(got))

	_, err = Dial(ctx, `\\server\pipe\x`)
	assert.ErrorIs(t, err, ErrInvalidName)
}
