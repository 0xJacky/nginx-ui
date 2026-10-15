package template

import (
	"regexp"
	"strings"

	"github.com/0xJacky/Nginx-UI/internal/nginx"
	"github.com/pkg/errors"
)

var (
	reUnsafeNginxValue = regexp.MustCompile(`[;{}\r\n]`)
	reNginxSize        = regexp.MustCompile(`^\d+[kKmMgG]?$`)
)

// IsSafeNginxValue reports whether value can be written as directive
// parameters without ending the directive or opening a block: it is not empty,
// has no surrounding whitespace and holds no `;`, braces or line breaks.
func IsSafeNginxValue(value string) bool {
	return value == strings.TrimSpace(value) && value != "" && !reUnsafeNginxValue.MatchString(value)
}

// IsSafeNginxToken is IsSafeNginxValue for a single parameter: it also
// rejects spaces and tabs.
func IsSafeNginxToken(value string) bool {
	return IsSafeNginxValue(value) && !strings.ContainsAny(value, " \t")
}

// IsNginxSize reports whether value is an nginx size such as 0, 512k or 64m.
func IsNginxSize(value string) bool {
	return reNginxSize.MatchString(value)
}

// LetsEncryptLocation returns the HTTP-01 challenge location parsed from the
// letsencrypt.conf block template. It proxies the challenge to Nginx UI.
func LetsEncryptLocation() (*nginx.NgxLocation, error) {
	block, err := ParseTemplate("block", "letsencrypt.conf", nil)
	if err != nil {
		return nil, err
	}
	if len(block.Locations) == 0 {
		return nil, errors.New("letsencrypt.conf contains no location")
	}
	return block.Locations[0], nil
}
