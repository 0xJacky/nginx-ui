package certificate

import (
	"io"
	"net/http"

	"github.com/0xJacky/Nginx-UI/internal/cert"
	"github.com/gin-gonic/gin"
	"github.com/uozi-tech/cosy"
)

// ParseCertificateArchive reads an uploaded zip and returns the certificate
// and private key found in it. Nothing is written to disk; the editor fills
// its fields from the response and the user saves as usual.
func ParseCertificateArchive(c *gin.Context) {
	fileHeader, err := c.FormFile("file")
	if err != nil {
		cosy.ErrHandler(c, cosy.WrapErrorWithParams(cert.ErrInvalidCertificateArchive, err.Error()))
		return
	}
	if fileHeader.Size > cert.MaxCertificateArchiveSize {
		cosy.ErrHandler(c, cert.ErrCertificateArchiveTooLarge)
		return
	}

	file, err := fileHeader.Open()
	if err != nil {
		cosy.ErrHandler(c, cosy.WrapErrorWithParams(cert.ErrInvalidCertificateArchive, err.Error()))
		return
	}
	defer file.Close()

	data, err := io.ReadAll(io.LimitReader(file, cert.MaxCertificateArchiveSize+1))
	if err != nil {
		cosy.ErrHandler(c, cosy.WrapErrorWithParams(cert.ErrInvalidCertificateArchive, err.Error()))
		return
	}

	result, err := cert.ParseCertificateArchive(data)
	if err != nil {
		cosy.ErrHandler(c, err)
		return
	}

	c.JSON(http.StatusOK, result)
}
