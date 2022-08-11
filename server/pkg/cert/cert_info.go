package cert

import (
	"crypto/tls"
	"crypto/x509"
	"github.com/pkg/errors"
	"io"
	"log"
	"net"
	"net/http"
	"time"
)

func GetCertInfo(domain string) (key *x509.Certificate, err error) {

	var response *http.Response

	client := &http.Client{
		Transport: &http.Transport{
			DialContext: (&net.Dialer{
				Timeout: 5 * time.Second,
			}).DialContext,
			DisableKeepAlives: true,
			TLSClientConfig:   &tls.Config{InsecureSkipVerify: true},
		},
		Timeout: 5 * time.Second,
	}

	response, err = client.Get("https://" + domain)

	if err != nil {
		err = errors.Wrap(err, "get cert info error")
		return
	}

	defer func(Body io.ReadCloser) {
		err = Body.Close()
		if err != nil {
			log.Println(err)
			return
		}
	}(response.Body)

	key = response.TLS.PeerCertificates[0]

	return
}
