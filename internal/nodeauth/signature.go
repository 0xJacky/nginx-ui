package nodeauth

import (
	"crypto/ed25519"
	"crypto/hkdf"
	"crypto/hmac"
	"crypto/rand"
	"crypto/sha256"
	"crypto/subtle"
	"encoding/base64"
	"errors"
	"fmt"
	"net/http"
	"strconv"
	"strings"
	"time"

	"github.com/0xJacky/Nginx-UI/model"
	"github.com/0xJacky/Nginx-UI/settings"
	"gorm.io/gorm"
)

const (
	signatureLabel             = "nginxui"
	signatureTag               = "nginx-ui-node-v1"
	signatureLifetime          = 60 * time.Second
	signatureClockSkew         = 60 * time.Second
	rotationRecoveryGrace      = 10 * time.Minute
	CredentialIDHeader         = "X-Nginx-UI-Credential-ID"
	TargetInstanceHeader       = "X-Nginx-UI-Target-Instance"
	contentDigestHeader        = "Content-Digest"
	signatureInputHeader       = "Signature-Input"
	signatureHeader            = "Signature"
	coveredComponentParameters = "(\"@method\" \"@path\" \"@query\" \"content-digest\" \"x-nginx-ui-credential-id\" \"x-nginx-ui-target-instance\")"

	signatureAlgorithmEd25519 = "ed25519"
	signatureAlgorithmHMAC    = "hmac-sha256"

	// sharedSecretKeyID marks a signature keyed by the configured node secret
	// rather than by a paired credential. It fills both identifier slots of the
	// envelope because a shared secret is not bound to a credential record or to
	// one target instance.
	sharedSecretKeyID   = "node-secret"
	sharedSecretKeyInfo = "nginx-ui/node-shared-secret/hmac-sha256/v1"
)

var defaultReplayCache = NewReplayCache(maxReplayItems)

type signatureMetadata struct {
	created      int64
	expires      int64
	nonce        string
	credentialID string
	algorithm    string
	parameters   string
}

func SignRequestWithKey(request *http.Request, credentialID, targetInstanceID string, privateKey ed25519.PrivateKey, now time.Time) error {
	if len(privateKey) != ed25519.PrivateKeySize {
		return errors.New("invalid Ed25519 private key")
	}
	return signRequest(request, credentialID, targetInstanceID, signatureAlgorithmEd25519, now,
		func(base []byte) ([]byte, error) {
			return ed25519.Sign(privateKey, base), nil
		})
}

// SignRequestWithSharedSecret authenticates a request with the node secret both
// instances already hold, without ever putting that secret on the wire.
func SignRequestWithSharedSecret(request *http.Request, secret []byte, now time.Time) error {
	if len(secret) == 0 {
		return errors.New("node secret is unavailable")
	}
	return signRequest(request, sharedSecretKeyID, sharedSecretKeyID, signatureAlgorithmHMAC, now,
		func(base []byte) ([]byte, error) {
			return sharedSecretSignature(secret, base)
		})
}

func signRequest(request *http.Request, credentialID, targetInstanceID, algorithm string, now time.Time,
	sign func(base []byte) ([]byte, error),
) error {
	if !validIdentifier(credentialID) || !validIdentifier(targetInstanceID) {
		return errors.New("invalid node signature identifier")
	}

	contentDigest, err := stageRequestBody(request)
	if err != nil {
		return err
	}

	nonceBytes := make([]byte, 16)
	if _, err := rand.Read(nonceBytes); err != nil {
		CloseStagedBody(request)
		return fmt.Errorf("generate node request nonce: %w", err)
	}
	created := now.Unix()
	expires := now.Add(signatureLifetime).Unix()
	nonce := base64.RawURLEncoding.EncodeToString(nonceBytes)
	parameters := fmt.Sprintf(
		"%s;created=%d;expires=%d;nonce=\"%s\";keyid=\"%s\";alg=\"%s\";tag=\"%s\"",
		coveredComponentParameters,
		created,
		expires,
		nonce,
		credentialID,
		algorithm,
		signatureTag,
	)

	request.Header.Del("X-Node-Secret")
	request.Header.Set(contentDigestHeader, contentDigest)
	request.Header.Set(CredentialIDHeader, credentialID)
	request.Header.Set(TargetInstanceHeader, targetInstanceID)
	request.Header.Set(signatureInputHeader, signatureLabel+"="+parameters)
	signature, err := sign([]byte(buildSignatureBase(request, parameters)))
	if err != nil {
		CloseStagedBody(request)
		return err
	}
	request.Header.Set(signatureHeader, signatureLabel+"=:"+base64.StdEncoding.EncodeToString(signature)+":")
	return nil
}

func sharedSecretSignature(secret, base []byte) ([]byte, error) {
	key, err := hkdf.Key(sha256.New, secret, []byte(signatureTag), sharedSecretKeyInfo, 32)
	if err != nil {
		return nil, fmt.Errorf("derive node secret signing key: %w", err)
	}
	mac := hmac.New(sha256.New, key)
	_, _ = mac.Write(base)
	return mac.Sum(nil), nil
}

func VerifyRequest(request *http.Request) (*Principal, error) {
	return verifyRequest(request, model.UseDB(), time.Now(), defaultReplayCache)
}

func verifyRequest(request *http.Request, database *gorm.DB, now time.Time, replayCache *ReplayCache) (*Principal, error) {
	if request == nil || database == nil {
		return nil, errors.New("node signature verifier is unavailable")
	}
	if request.URL.Query().Has("node_secret") {
		return nil, errors.New("node credentials are not accepted in query parameters")
	}

	metadata, err := parseSignatureInput(request.Header)
	if err != nil {
		return nil, err
	}
	if credentialID := singleHeaderValue(request.Header, CredentialIDHeader); credentialID == "" || credentialID != metadata.credentialID {
		return nil, errors.New("node signature credential identifier mismatch")
	}
	if err := validateSignatureTime(metadata, now); err != nil {
		return nil, err
	}
	if metadata.algorithm == signatureAlgorithmHMAC {
		return verifySharedSecretRequest(request, metadata, now, replayCache)
	}
	if target := singleHeaderValue(request.Header, TargetInstanceHeader); target == "" || target != settings.NodeSettings.InstanceID {
		return nil, errors.New("node signature target does not match this instance")
	}

	var credential model.NodeControllerCredential
	if err := database.Where(
		"credential_id = ? AND revoked_at IS NULL AND status IN ?",
		metadata.credentialID,
		[]string{model.NodeCredentialStatusActive, model.NodeCredentialStatusRotating},
	).First(&credential).Error; err != nil {
		return nil, errors.New("node signing credential is unknown or revoked")
	}

	contentDigest, err := stageRequestBody(request)
	if err != nil {
		return nil, err
	}
	receivedDigest := singleHeaderValue(request.Header, contentDigestHeader)
	if receivedDigest == "" || subtle.ConstantTimeCompare([]byte(receivedDigest), []byte(contentDigest)) != 1 {
		return nil, errors.New("node request content digest mismatch")
	}

	signature, err := parseSignature(request.Header, ed25519.SignatureSize)
	if err != nil {
		return nil, err
	}
	signatureBase := []byte(buildSignatureBase(request, metadata.parameters))
	if !verifyCredentialSignature(&credential, signatureBase, signature, now) {
		return nil, errors.New("node request signature is invalid")
	}

	if replayCache == nil || !replayCache.Use(metadata.credentialID+":"+metadata.nonce, now) {
		return nil, errors.New("node request nonce was already used")
	}

	if err := database.Model(&model.NodeControllerCredential{}).
		Where("id = ?", credential.ID).
		Update("last_used_at", now).Error; err != nil {
		return nil, fmt.Errorf("record node credential use: %w", err)
	}
	return &Principal{
		CredentialID:         credential.CredentialID,
		ControllerInstanceID: credential.ControllerInstanceID,
		AuthMethod:           model.NodeAuthMethodPaired,
	}, nil
}

// verifySharedSecretRequest authenticates a request signed with the node secret
// both instances hold. The secret itself never travels, so an observer of the
// link learns nothing reusable, and the covered components plus the nonce make
// the signature useless for anything other than the exact request it was made
// for.
func verifySharedSecretRequest(request *http.Request, metadata signatureMetadata, now time.Time,
	replayCache *ReplayCache,
) (*Principal, error) {
	// A shared secret is not bound to a credential record or to one instance, so
	// both identifier slots have to carry the sentinel rather than real values.
	if metadata.credentialID != sharedSecretKeyID ||
		singleHeaderValue(request.Header, TargetInstanceHeader) != sharedSecretKeyID {
		return nil, errors.New("node signature identifiers do not match a shared secret")
	}
	secret := strings.TrimSpace(settings.NodeSettings.Secret)
	if secret == "" {
		return nil, errors.New("node secret is not configured")
	}

	contentDigest, err := stageRequestBody(request)
	if err != nil {
		return nil, err
	}
	receivedDigest := singleHeaderValue(request.Header, contentDigestHeader)
	if receivedDigest == "" || subtle.ConstantTimeCompare([]byte(receivedDigest), []byte(contentDigest)) != 1 {
		return nil, errors.New("node request content digest mismatch")
	}

	signature, err := parseSignature(request.Header, sha256.Size)
	if err != nil {
		return nil, err
	}
	expected, err := sharedSecretSignature([]byte(secret), []byte(buildSignatureBase(request, metadata.parameters)))
	if err != nil {
		return nil, err
	}
	if subtle.ConstantTimeCompare(signature, expected) != 1 {
		return nil, errors.New("node request signature is invalid")
	}
	if replayCache == nil || !replayCache.Use(sharedSecretKeyID+":"+metadata.nonce, now) {
		return nil, errors.New("node request nonce was already used")
	}

	return &Principal{
		CredentialID:         "legacy",
		ControllerInstanceID: "legacy",
		AuthMethod:           model.NodeAuthMethodLegacy,
	}, nil
}

func parseSignatureInput(header http.Header) (signatureMetadata, error) {
	value := singleHeaderValue(header, signatureInputHeader)
	prefix := signatureLabel + "=" + coveredComponentParameters + ";created="
	if value == "" || !strings.HasPrefix(value, prefix) {
		return signatureMetadata{}, errors.New("node signature input is missing or unsupported")
	}

	remaining := strings.TrimPrefix(value, signatureLabel+"="+coveredComponentParameters)
	created, remaining, err := consumeIntegerParameter(remaining, "created")
	if err != nil {
		return signatureMetadata{}, err
	}
	expires, remaining, err := consumeIntegerParameter(remaining, "expires")
	if err != nil {
		return signatureMetadata{}, err
	}
	nonce, remaining, err := consumeQuotedParameter(remaining, "nonce")
	if err != nil || len(nonce) != 22 || !validIdentifier(nonce) {
		return signatureMetadata{}, errors.New("invalid node signature nonce")
	}
	credentialID, remaining, err := consumeQuotedParameter(remaining, "keyid")
	if err != nil || !validIdentifier(credentialID) {
		return signatureMetadata{}, errors.New("invalid node signature key identifier")
	}
	algorithm, remaining, err := consumeQuotedParameter(remaining, "alg")
	if err != nil || (algorithm != signatureAlgorithmEd25519 && algorithm != signatureAlgorithmHMAC) {
		return signatureMetadata{}, errors.New("unsupported node signature algorithm")
	}
	tag, remaining, err := consumeQuotedParameter(remaining, "tag")
	if err != nil || tag != signatureTag || remaining != "" {
		return signatureMetadata{}, errors.New("invalid node signature tag")
	}

	return signatureMetadata{
		created:      created,
		expires:      expires,
		nonce:        nonce,
		credentialID: credentialID,
		algorithm:    algorithm,
		parameters:   strings.TrimPrefix(value, signatureLabel+"="),
	}, nil
}

func consumeIntegerParameter(input, name string) (int64, string, error) {
	prefix := ";" + name + "="
	if !strings.HasPrefix(input, prefix) {
		return 0, input, fmt.Errorf("missing node signature parameter %s", name)
	}
	input = strings.TrimPrefix(input, prefix)
	end := strings.IndexByte(input, ';')
	if end < 1 {
		return 0, input, fmt.Errorf("invalid node signature parameter %s", name)
	}
	value, err := strconv.ParseInt(input[:end], 10, 64)
	if err != nil || value < 0 {
		return 0, input, fmt.Errorf("invalid node signature parameter %s", name)
	}
	return value, input[end:], nil
}

func consumeQuotedParameter(input, name string) (string, string, error) {
	prefix := ";" + name + "=\""
	if !strings.HasPrefix(input, prefix) {
		return "", input, fmt.Errorf("missing node signature parameter %s", name)
	}
	input = strings.TrimPrefix(input, prefix)
	end := strings.IndexByte(input, '"')
	if end < 0 {
		return "", input, fmt.Errorf("invalid node signature parameter %s", name)
	}
	value := input[:end]
	if strings.ContainsAny(value, "\\\r\n") {
		return "", input, fmt.Errorf("invalid node signature parameter %s", name)
	}
	return value, input[end+1:], nil
}

func parseSignature(header http.Header, expectedSize int) ([]byte, error) {
	value := singleHeaderValue(header, signatureHeader)
	prefix := signatureLabel + "=:"
	if !strings.HasPrefix(value, prefix) || !strings.HasSuffix(value, ":") {
		return nil, errors.New("node signature is missing or invalid")
	}
	encoded := strings.TrimSuffix(strings.TrimPrefix(value, prefix), ":")
	signature, err := base64.StdEncoding.DecodeString(encoded)
	if err != nil || len(signature) != expectedSize {
		return nil, errors.New("node signature is missing or invalid")
	}
	return signature, nil
}

func validateSignatureTime(metadata signatureMetadata, now time.Time) error {
	created := time.Unix(metadata.created, 0)
	expires := time.Unix(metadata.expires, 0)
	if !expires.After(created) || expires.Sub(created) > signatureLifetime {
		return errors.New("node signature lifetime is invalid")
	}
	if created.After(now.Add(signatureClockSkew)) {
		return errors.New("node signature creation time is in the future")
	}
	if expires.Before(now.Add(-signatureClockSkew)) {
		return errors.New("node signature is expired")
	}
	return nil
}

func verifyCredentialSignature(credential *model.NodeControllerCredential, base, signature []byte, now time.Time) bool {
	if len(credential.PublicKey) == ed25519.PublicKeySize && ed25519.Verify(ed25519.PublicKey(credential.PublicKey), base, signature) {
		return true
	}
	return credential.PreviousValidUntil != nil &&
		now.Before(*credential.PreviousValidUntil) &&
		len(credential.PreviousPublicKey) == ed25519.PublicKeySize &&
		ed25519.Verify(ed25519.PublicKey(credential.PreviousPublicKey), base, signature)
}

func buildSignatureBase(request *http.Request, parameters string) string {
	path := request.URL.EscapedPath()
	if path == "" {
		path = "/"
	}
	query := "?" + request.URL.RawQuery
	return fmt.Sprintf(
		"\"@method\": %s\n\"@path\": %s\n\"@query\": %s\n\"content-digest\": %s\n\"x-nginx-ui-credential-id\": %s\n\"x-nginx-ui-target-instance\": %s\n\"@signature-params\": %s",
		strings.ToUpper(request.Method),
		path,
		query,
		request.Header.Get(contentDigestHeader),
		request.Header.Get(CredentialIDHeader),
		request.Header.Get(TargetInstanceHeader),
		parameters,
	)
}

func singleHeaderValue(header http.Header, name string) string {
	values := header.Values(name)
	if len(values) != 1 {
		return ""
	}
	return strings.TrimSpace(values[0])
}

func validIdentifier(value string) bool {
	if value == "" || len(value) > 128 {
		return false
	}
	for _, char := range value {
		if (char >= 'a' && char <= 'z') ||
			(char >= 'A' && char <= 'Z') ||
			(char >= '0' && char <= '9') ||
			char == '-' || char == '_' {
			continue
		}
		return false
	}
	return true
}

func CloseStagedBody(request *http.Request) {
	if request == nil || request.Body == nil {
		return
	}
	if _, ok := request.Body.(*temporaryRequestBody); ok {
		_ = request.Body.Close()
		request.Body = http.NoBody
	}
}
