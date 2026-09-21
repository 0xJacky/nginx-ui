package cert

import (
	"context"
	"errors"
	"log/slog"
	"os"
	"runtime"
	"strings"
	"time"

	"github.com/0xJacky/Nginx-UI/internal/cert/dns"
	"github.com/0xJacky/Nginx-UI/internal/nginx"
	"github.com/0xJacky/Nginx-UI/internal/translation"
	"github.com/0xJacky/Nginx-UI/internal/transport"
	"github.com/0xJacky/Nginx-UI/model"
	"github.com/0xJacky/Nginx-UI/query"
	"github.com/0xJacky/Nginx-UI/settings"
	"github.com/go-acme/lego/v5/challenge/http01"
	"github.com/go-acme/lego/v5/lego"
	legolog "github.com/go-acme/lego/v5/log"
	"github.com/uozi-tech/cosy"
	"github.com/uozi-tech/cosy/logger"
	cSettings "github.com/uozi-tech/cosy/settings"
)

const (
	HTTP01 = "http01"
	DNS01  = "dns01"

	// http01ProbeOverallTimeout bounds the whole HTTP-01 route probe; each
	// request is bounded separately by http01ProbeRequestTimeout.
	http01ProbeOverallTimeout = 30 * time.Second

	// envDisableCNAMESupport tells lego not to follow the CNAME of the
	// challenge record while deriving the challenge name.
	envDisableCNAMESupport = "LEGO_DISABLE_CNAME_SUPPORT"
)

// InitChallengeEnv pins the lego challenge environment for the whole process.
//
// CNAME delegation for DNS-01 is resolved by the plugin that owns the
// provider, per certificate and with the user's own resolvers, so the core
// must not resolve it a second time while deriving the challenge name.
// dns01.GetChallengeInfo reads the variable on every call, and the plugin
// supervisor strips it from the environment plugin processes inherit.
func InitChallengeEnv() {
	if err := os.Setenv(envDisableCNAMESupport, "true"); err != nil {
		logger.Error(err)
	}
}

func IssueCert(payload *ConfigPayload, certLogger *Logger) error {
	lock()
	defer unlock()
	ctx := context.Background()
	defer func() {
		if err := recover(); err != nil {
			buf := make([]byte, 1024)
			runtime.Stack(buf, false)
			logger.Errorf("%s\n%s", err, buf)
		}
	}()
	payload.KeyType = payload.GetKeyType()
	if err := NormalizeAndValidateIdentifiers(payload); err != nil {
		return err
	}
	if isHTTP01ChallengeMethod(payload.ChallengeMethod) && payload.ConfigName != "" {
		if err := verifyHTTP01ChallengeRoute(payload, certLogger); err != nil {
			return err
		}
	}

	// initial a channelWriter to receive logs
	cw := NewChannelWriter()
	defer close(cw.Ch)

	// Hijack the (logger) of lego
	oldLogger := legolog.Default()
	legolog.SetDefault(slog.New(slog.NewTextHandler(cw, nil)))
	// Restore the original logger, fix #876
	defer func() {
		legolog.SetDefault(oldLogger)
	}()

	certLogger.Info(translation.C("[Nginx UI] Preparing lego configurations"))
	user, err := payload.GetACMEUser()
	if err != nil {
		return cosy.WrapErrorWithParams(ErrGetACMEUser, err.Error())
	}

	certLogger.Info(translation.C("[Nginx UI] ACME User: %{name}, Email: %{email}, CA Dir: %{caDir}", map[string]any{
		"name":  user.Name,
		"email": user.Email,
		"caDir": user.CADir,
	}))

	// Start a goroutine to fetch and process logs from channel
	go func() {
		for msg := range cw.Ch {
			certLogger.Info(translation.C(string(msg)))
		}
	}()

	config := lego.NewConfig(user)

	config.CADirURL = user.CADir

	// Skip TLS check
	if config.HTTPClient != nil {
		t, err := transport.NewTransport(
			transport.WithProxy(user.Proxy))
		if err != nil {
			return cosy.WrapErrorWithParams(ErrNewTransport, err.Error())
		}
		config.HTTPClient.Transport = t
	}

	certLogger.Info(translation.C("[Nginx UI] Creating client facilitates communication with the CA server"))
	// A client facilitates communication with the CA server.
	client, err := lego.NewClient(config)
	if err != nil {
		return cosy.WrapErrorWithParams(ErrNewLegoClient, err.Error())
	}
	if err = resolveCertificateProfile(payload, client.GetServerMetadata().Profiles); err != nil {
		return err
	}

	switch payload.ChallengeMethod {
	default:
		fallthrough
	case HTTP01:
		certLogger.Info(translation.C("[Nginx UI] Setting HTTP01 challenge provider"))
		err = client.Challenge.SetHTTP01Provider(
			http01.NewProviderServer("",
				settings.CertSettings.HTTPChallengePort,
			),
		)
	case DNS01:
		credential, credentialErr := query.DnsCredential.FirstByID(payload.DNSCredentialID)
		if credentialErr != nil {
			return cosy.WrapErrorWithParams(ErrGetDNSCredential, credentialErr.Error())
		}
		if credential.Config == nil || credential.Config.Configuration == nil {
			return ErrEnvironmentConfigurationIsEmpty
		}

		code := credential.ProviderCode
		if code == "" {
			code = credential.Config.Code
		}

		certLogger.Info(translation.C("[Nginx UI] Setting DNS01 challenge provider"))
		// Every vendor call, the propagation check and the CNAME delegation
		// happen inside the plugin process that owns the provider code.
		provider, options, release, providerErr := dns.NewChallengeProvider(ctx, code,
			*credential.Config.Configuration, payload.EffectiveChallengeConfig())
		if errors.Is(providerErr, dns.ErrProviderNotFound) {
			return cosy.WrapErrorWithParams(ErrNoDNS01Provider, code)
		}
		defer release()
		if providerErr != nil {
			return cosy.WrapErrorWithParams(ErrNewDNSChallengeProvider, providerErr.Error())
		}

		err = client.Challenge.SetDNS01Provider(provider, options...)
	}

	if err != nil {
		return cosy.WrapErrorWithParams(ErrChallengeError, err.Error())
	}

	// Backup current certificate and key if RevokeOld is true
	var oldResource *model.CertificateResource

	if payload.RevokeOld && payload.Resource != nil && payload.Resource.Certificate != nil {
		certLogger.Info(translation.C("[Nginx UI] Backing up current certificate for later revocation"))

		// Save a copy of the old certificate and key
		oldResource = &model.CertificateResource{
			Resource:    payload.Resource.Resource,
			Certificate: payload.Resource.Certificate,
			PrivateKey:  payload.Resource.PrivateKey,
		}
	}

	if canUseLegoRenew(payload) &&
		time.Since(payload.NotBefore).Hours()/24 <= 21 &&
		payload.Resource != nil && payload.Resource.Certificate != nil {
		err = renew(payload, client, certLogger)
		if err != nil {
			return err
		}
	} else {
		err = obtain(payload, client, certLogger)
		if err != nil {
			return err
		}
	}

	certLogger.Info(translation.C("[Nginx UI] Reloading nginx"))

	nginx.Reload()

	certLogger.Info(translation.C("[Nginx UI] Finished"))

	if payload.GetCertificatePath() == cSettings.ServerSettings.SSLCert &&
		payload.GetCertificateKeyPath() == cSettings.ServerSettings.SSLKey {
		ReloadServerTLSCertificate()
	}

	// Revoke old certificate if requested and we have a backup
	if payload.RevokeOld && oldResource != nil && len(oldResource.Certificate) > 0 {
		certLogger.Info(translation.C("[Nginx UI] Revoking old certificate"))

		// Create a payload for revocation using old certificate
		revokePayload := &ConfigPayload{
			CertID:          payload.CertID,
			ServerName:      payload.ServerName,
			ChallengeMethod: payload.ChallengeMethod,
			DNSCredentialID: payload.DNSCredentialID,
			ACMEUserID:      payload.ACMEUserID,
			KeyType:         payload.KeyType,
			Resource:        oldResource,
		}

		// Revoke the old certificate
		err = revoke(revokePayload, client, certLogger)
		if err != nil {
			return err
		}
	}

	// Wait log to be written
	time.Sleep(2 * time.Second)

	return nil
}

// isHTTP01ChallengeMethod reports whether method selects HTTP-01, which is
// also the default when no method is recorded.
func isHTTP01ChallengeMethod(method string) bool {
	return method == "" || method == HTTP01
}

// verifyHTTP01ChallengeRoute runs the active loopback probe while IssueCert
// holds the certificate lock and logs one line per domain. It never blocks
// issuance on a route result: the probe can only see the local Nginx, while
// the CA verifies the route itself. The only error it returns is an
// unbindable challenge port (50059), because lego would fail the same way.
func verifyHTTP01ChallengeRoute(payload *ConfigPayload, certLogger *Logger) error {
	certLogger.Info(translation.C("[Nginx UI] Checking HTTP01 challenge route for %{domains}", map[string]any{
		"domains": strings.Join(payload.ServerName, ", "),
	}))

	ctx, cancel := context.WithTimeout(context.Background(), http01ProbeOverallTimeout)
	defer cancel()
	results, err := probeHTTP01Routes(ctx, payload.ServerName, WithHTTP01ProbeConfigName(payload.ConfigName))
	if err != nil {
		if isHTTP01ChallengePortUnavailable(err) {
			return err
		}
		certLogger.Info(translation.C("[Nginx UI] HTTP01 challenge route check could not run: %{error}", map[string]any{
			"error": err.Error(),
		}))
		return nil
	}
	if HTTP01ProbeSkipped(results) {
		certLogger.Info(translation.C("[Nginx UI] HTTP01 challenge route check skipped: %{reason}", map[string]any{
			"reason": results[0].SkipReason,
		}))
		return nil
	}
	for _, result := range results {
		switch result.Status {
		case HTTP01ProbeStatusSuccess:
			certLogger.Info(translation.C("[Nginx UI] HTTP01 challenge route reachable for %{domain} via %{target}", map[string]any{
				"domain": result.Domain,
				"target": result.Target,
			}))
		case HTTP01ProbeStatusWarning:
			certLogger.Info(translation.C("[Nginx UI] HTTP01 challenge route cannot be verified locally for %{domain}: %{reason}", map[string]any{
				"domain": result.Domain,
				"reason": describeHTTP01ProbeFailure(result),
			}))
		default:
			certLogger.Info(translation.C("[Nginx UI] HTTP01 challenge route check failed for %{domain}: %{error}. Issuance continues; the certificate authority will verify the route itself", map[string]any{
				"domain": result.Domain,
				"error":  describeHTTP01ProbeFailureWithCause(result),
			}))
		}
	}
	return nil
}

// isHTTP01ChallengePortUnavailable reports whether err is the 50059 error of
// a challenge port the probe could not bind.
func isHTTP01ChallengePortUnavailable(err error) bool {
	var got, want *cosy.Error
	if !errors.As(err, &got) || !errors.As(ErrHTTP01ChallengePortUnavailable, &want) {
		return false
	}
	return got.Scope == want.Scope && got.Code == want.Code
}

func canUseLegoRenew(payload *ConfigPayload) bool {
	if payload == nil {
		return true
	}

	// lego.RenewOptions does not expose EnableCommonName or ReplacesCertID,
	// so use the obtain flow when either option is required.
	return !payload.EnableCommonName && payload.ReplacesCertID == ""
}
