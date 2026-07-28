package nodeauth

import (
	"crypto/sha256"
	"encoding/base64"
	"fmt"
	"hash"
	"io"
	"net/http"
	"os"
)

type temporaryRequestBody struct {
	*os.File
	path string
}

func (body *temporaryRequestBody) Close() error {
	closeErr := body.File.Close()
	removeErr := os.Remove(body.path)
	if closeErr != nil {
		return closeErr
	}
	if removeErr != nil && !os.IsNotExist(removeErr) {
		return removeErr
	}
	return nil
}

func stageRequestBody(request *http.Request) (string, error) {
	digest := sha256.New()
	if request.Body == nil || request.Body == http.NoBody {
		return formatContentDigest(digest), nil
	}

	temporary, err := os.CreateTemp("", "nginx-ui-node-request-body-*")
	if err != nil {
		return "", fmt.Errorf("stage signed request body: %w", err)
	}
	path := temporary.Name()
	cleanup := func() {
		_ = temporary.Close()
		_ = os.Remove(path)
	}

	size, err := io.Copy(io.MultiWriter(temporary, digest), request.Body)
	closeErr := request.Body.Close()
	if err != nil {
		cleanup()
		return "", fmt.Errorf("stage signed request body: %w", err)
	}
	if closeErr != nil {
		cleanup()
		return "", fmt.Errorf("close original signed request body: %w", closeErr)
	}
	if err := temporary.Sync(); err != nil {
		cleanup()
		return "", fmt.Errorf("sync signed request body: %w", err)
	}
	if _, err := temporary.Seek(0, io.SeekStart); err != nil {
		cleanup()
		return "", fmt.Errorf("rewind signed request body: %w", err)
	}

	request.Body = &temporaryRequestBody{File: temporary, path: path}
	request.ContentLength = size
	return formatContentDigest(digest), nil
}

func formatContentDigest(digest hash.Hash) string {
	return "sha-256=:" + base64.StdEncoding.EncodeToString(digest.Sum(nil)) + ":"
}
