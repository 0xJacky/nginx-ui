package transport

import (
	"net/http"
	"net/url"
	"testing"

	"github.com/0xJacky/Nginx-UI/settings"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestCreatesTransportWithDefaultSettings(t *testing.T) {
	transport, err := NewTransport()
	require.NoError(t, err)
	assert.NotNil(t, transport)
	assert.ObjectsAreEqual(http.ProxyFromEnvironment, transport.Proxy)
	assert.Equal(t, settings.ServerSettings.InsecureSkipVerify, transport.TLSClientConfig.InsecureSkipVerify)
}

func TestCreatesTransportWithCustomProxy(t *testing.T) {
	proxyUrl := "https://proxy.example.com"
	transport, err := NewTransport(WithProxy(proxyUrl))
	require.NoError(t, err)
	assert.NotNil(t, transport)
	parsedProxy, _ := url.Parse(proxyUrl)
	assert.ObjectsAreEqual(http.ProxyURL(parsedProxy), transport.Proxy)
}

func TestCreatesTransportWithInvalidProxyUrl(t *testing.T) {
	invalidProxyUrl := "https://[::1]:namedport"
	transport, err := NewTransport(WithProxy(invalidProxyUrl))
	assert.Error(t, err)
	assert.Nil(t, transport)
}

func TestCreatesTransportWithEmptyProxyUrl(t *testing.T) {
	transport, err := NewTransport(WithProxy(""))
	require.NoError(t, err)
	assert.NotNil(t, transport)
	assert.ObjectsAreEqual(http.ProxyFromEnvironment, transport.Proxy)
}
