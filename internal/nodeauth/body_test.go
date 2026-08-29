package nodeauth

import (
	"errors"
	"io"
	"net/http"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

type trackingReadCloser struct {
	reader     io.Reader
	bytesRead  int
	isClosed   bool
	failOnRead bool
}

func (body *trackingReadCloser) Read(buffer []byte) (int, error) {
	if body.failOnRead {
		return 0, errors.New("request body must not be read")
	}
	if body.reader == nil {
		return 0, io.EOF
	}
	count, err := body.reader.Read(buffer)
	body.bytesRead += count
	return count, err
}

func (body *trackingReadCloser) Close() error {
	body.isClosed = true
	return nil
}

func TestStageRequestBodyRejectsDeclaredOversizeWithoutReading(t *testing.T) {
	body := &trackingReadCloser{failOnRead: true}
	request := &http.Request{
		Body:          body,
		ContentLength: 5,
	}

	_, err := stageRequestBodyWithLimit(request, 4)

	require.ErrorIs(t, err, errRequestBodyTooLarge)
	assert.Zero(t, body.bytesRead)
	assert.True(t, body.isClosed)
}

func TestStageRequestBodyLimitsUnknownLengthStream(t *testing.T) {
	temporaryDirectory := t.TempDir()
	t.Setenv("TMPDIR", temporaryDirectory)
	body := &trackingReadCloser{reader: strings.NewReader("12345")}
	request := &http.Request{
		Body:          body,
		ContentLength: -1,
	}

	_, err := stageRequestBodyWithLimit(request, 4)

	require.ErrorIs(t, err, errRequestBodyTooLarge)
	assert.Equal(t, 5, body.bytesRead)
	assert.True(t, body.isClosed)
	assert.Empty(t, stagedBodyFiles(t, temporaryDirectory))
}

func TestStageRequestBodyAcceptsExactStreamingLimit(t *testing.T) {
	temporaryDirectory := t.TempDir()
	t.Setenv("TMPDIR", temporaryDirectory)
	request := &http.Request{
		Body:          io.NopCloser(strings.NewReader("1234")),
		ContentLength: -1,
	}

	digest, err := stageRequestBodyWithLimit(request, 4)
	require.NoError(t, err)
	assert.Equal(t, "sha-256=:A6xnQhbz4Vx2HuGl4lXwZ5U2I8iziLRFnhP5eNfIRvQ=:", digest)
	assert.EqualValues(t, 4, request.ContentLength)
	assert.Len(t, stagedBodyFiles(t, temporaryDirectory), 1)

	content, err := io.ReadAll(request.Body)
	require.NoError(t, err)
	assert.Equal(t, "1234", string(content))
	CloseStagedBody(request)
	assert.Empty(t, stagedBodyFiles(t, temporaryDirectory))
}

func stagedBodyFiles(t *testing.T, directory string) []string {
	t.Helper()
	files, err := filepath.Glob(filepath.Join(directory, "nginx-ui-node-request-body-*"))
	require.NoError(t, err)
	for _, file := range files {
		_, err = os.Stat(file)
		require.NoError(t, err)
	}
	return files
}
