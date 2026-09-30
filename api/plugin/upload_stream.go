package plugin

import (
	"errors"
	"io"
	"mime/multipart"
	"net/http"
	"os"
	"path/filepath"

	plugin "github.com/0xJacky/Nginx-UI/internal/plugin"
	"github.com/gin-gonic/gin"
	"github.com/uozi-tech/cosy"
)

const (
	// uploadFilePart is the form field that carries the package.
	uploadFilePart = "file"
	// maxUploadFieldSize caps one text field of an upload form.
	maxUploadFieldSize = 64 << 10
	// maxUploadFields caps how many text fields an upload form may carry.
	maxUploadFields = 16
	// uploadOverhead is the room the multipart framing and the text fields
	// get on top of the package itself.
	uploadOverhead = 1 << 20
)

// maxUploadSize is the largest package file accepted. The compressed archive
// never exceeds the uncompressed budget of a package.
var maxUploadSize int64 = plugin.MaxPackageSize

// uploadForm is a multipart request read in one streaming pass.
type uploadForm struct {
	// path is the stored package, empty when the request had no file part.
	path   string
	fields map[string]string
	// cleanup removes the stored package, it is always safe to call.
	cleanup func()
}

func errUploadFileMissing() error {
	return cosy.WrapErrorWithParams(plugin.ErrPackageInvalid, "file is missing")
}

// readUpload streams the multipart body: the file part goes straight to a
// temporary file and the small text fields are collected, in any order. Gin's
// FormFile is avoided because it buffers part of the body in memory.
func readUpload(c *gin.Context) (*uploadForm, error) {
	form := &uploadForm{fields: map[string]string{}, cleanup: func() {}}

	c.Request.Body = http.MaxBytesReader(c.Writer, c.Request.Body, maxUploadSize+uploadOverhead)
	reader, err := c.Request.MultipartReader()
	if err != nil {
		return nil, cosy.WrapErrorWithParams(plugin.ErrPackageInvalid, err.Error())
	}

	var dir string
	fail := func(err error) (*uploadForm, error) {
		if dir != "" {
			_ = os.RemoveAll(dir)
		}
		return nil, err
	}

	for {
		part, err := reader.NextPart()
		if errors.Is(err, io.EOF) {
			break
		}
		if err != nil {
			return fail(uploadReadError(err))
		}

		name := part.FormName()
		switch {
		case name == uploadFilePart && form.path == "":
			dir, err = os.MkdirTemp("", "nginx-ui-plugin-upload-")
			if err != nil {
				return fail(err)
			}
			target := filepath.Join(dir, "package.tar.gz")
			if err = storeUploadFile(part, target); err != nil {
				return fail(err)
			}
			form.path = target
			form.cleanup = func() { _ = os.RemoveAll(dir) }
		case part.FileName() != "" || name == "":
			// Other files are not part of the protocol.
			_, _ = io.Copy(io.Discard, part)
		default:
			if _, seen := form.fields[name]; !seen && len(form.fields) >= maxUploadFields {
				return fail(cosy.WrapErrorWithParams(plugin.ErrPackageInvalid, "too many form fields"))
			}
			value, err := io.ReadAll(io.LimitReader(part, maxUploadFieldSize+1))
			if err != nil {
				return fail(uploadReadError(err))
			}
			if len(value) > maxUploadFieldSize {
				return fail(cosy.WrapErrorWithParams(plugin.ErrPackageInvalid, "form field "+name+" is too large"))
			}
			form.fields[name] = string(value)
		}
	}
	return form, nil
}

// storeUploadFile copies one file part to target within the size limit.
func storeUploadFile(part *multipart.Part, target string) error {
	file, err := os.OpenFile(target, os.O_CREATE|os.O_EXCL|os.O_WRONLY, 0o600)
	if err != nil {
		return err
	}
	written, err := io.Copy(file, io.LimitReader(part, maxUploadSize+1))
	if closeErr := file.Close(); err == nil {
		err = closeErr
	}
	if err != nil {
		return uploadReadError(err)
	}
	if written > maxUploadSize {
		return plugin.ErrPackageTooLarge
	}
	return nil
}

// uploadReadError classifies a failure while reading the request body.
func uploadReadError(err error) error {
	var tooLarge *http.MaxBytesError
	if errors.As(err, &tooLarge) {
		return plugin.ErrPackageTooLarge
	}
	return cosy.WrapErrorWithParams(plugin.ErrPackageInvalid, err.Error())
}
