package plugin

import (
	"bytes"
	"mime/multipart"
	"net/http"
	"os"
	"strings"
	"testing"

	plugin "github.com/0xJacky/Nginx-UI/internal/plugin"
	"github.com/gin-gonic/gin"
	"github.com/stretchr/testify/require"
)

// streamContext builds a multipart request whose parts follow the given order:
// "file:<content>" adds the file part, "name=value" adds a text field.
func streamContext(t *testing.T, parts ...string) *gin.Context {
	t.Helper()

	var body bytes.Buffer
	writer := multipart.NewWriter(&body)
	for _, part := range parts {
		if content, ok := strings.CutPrefix(part, "file:"); ok {
			file, err := writer.CreateFormFile("file", "package.tar.gz")
			require.NoError(t, err)
			_, err = file.Write([]byte(content))
			require.NoError(t, err)
			continue
		}
		name, value, _ := strings.Cut(part, "=")
		require.NoError(t, writer.WriteField(name, value))
	}
	require.NoError(t, writer.Close())

	c, _ := newContext(http.MethodPost, "/api/plugins", &body, nil)
	c.Request.Header.Set("Content-Type", writer.FormDataContentType())
	return c
}

func TestReadUploadFileThenField(t *testing.T) {
	useUploadStore(t)
	form, err := readUpload(streamContext(t, "file:payload", "enable=true"))
	require.NoError(t, err)
	defer form.cleanup()

	data, err := os.ReadFile(form.path)
	require.NoError(t, err)
	require.Equal(t, "payload", string(data))
	require.Equal(t, "true", form.fields["enable"])
}

func TestReadUploadFieldThenFile(t *testing.T) {
	useUploadStore(t)
	form, err := readUpload(streamContext(t, "enable=true", "author_public_key=abc", "file:payload"))
	require.NoError(t, err)
	defer form.cleanup()

	data, err := os.ReadFile(form.path)
	require.NoError(t, err)
	require.Equal(t, "payload", string(data))
	require.Equal(t, "true", form.fields["enable"])
	require.Equal(t, "abc", form.fields["author_public_key"])
}

func TestReadUploadWithoutFilePart(t *testing.T) {
	useUploadStore(t)
	form, err := readUpload(streamContext(t, "enable=true"))
	require.NoError(t, err)
	defer form.cleanup()
	require.Empty(t, form.path)

	// The inspect endpoint turns that into an invalid package.
	c, recorder := newUploadContext(t, "/api/plugins/inspect", map[string]string{"enable": "true"}, "")
	InspectPlugin(c)
	require.NotEqual(t, http.StatusOK, recorder.Code)
	require.Contains(t, recorder.Body.String(), "file is missing")
}

func TestReadUploadRejectsOversizeFile(t *testing.T) {
	temp, _ := useUploadStore(t)
	previous := maxUploadSize
	maxUploadSize = 8
	t.Cleanup(func() { maxUploadSize = previous })

	form, err := readUpload(streamContext(t, "file:0123456789"))
	require.Nil(t, form)
	require.ErrorIs(t, err, plugin.ErrPackageTooLarge)

	leftovers, globErr := os.ReadDir(temp)
	require.NoError(t, globErr)
	require.Empty(t, leftovers, "the partial file is removed")
}

func TestReadUploadRejectsOversizeField(t *testing.T) {
	useUploadStore(t)
	form, err := readUpload(streamContext(t, "note="+strings.Repeat("x", maxUploadFieldSize+1), "file:payload"))
	require.Nil(t, form)
	require.Error(t, err)
}

func TestReadUploadRejectsNonMultipart(t *testing.T) {
	c, _ := newContext(http.MethodPost, "/api/plugins", strings.NewReader("{}"), nil)
	form, err := readUpload(c)
	require.Nil(t, form)
	require.Error(t, err)
}
