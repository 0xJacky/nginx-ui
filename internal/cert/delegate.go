package cert

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"path/filepath"
	"strings"
	"time"

	"github.com/0xJacky/Nginx-UI/internal/helper"
	"github.com/0xJacky/Nginx-UI/internal/nginx"
	"github.com/0xJacky/Nginx-UI/internal/nodeauth"
	"github.com/0xJacky/Nginx-UI/internal/translation"
	"github.com/0xJacky/Nginx-UI/model"
	"github.com/0xJacky/Nginx-UI/query"
	"github.com/go-acme/lego/v5/certcrypto"
	"github.com/uozi-tech/cosy"
)

// delegatedDirName below the ssl directory holds the certificates this
// instance issues for its nodes. No configuration of this instance loads them.
const delegatedDirName = "nodes"

const nodeRequestTimeout = 30 * time.Second

// DelegatedFilename keys the record of a certificate issued for a node. A
// configuration name is a file name and never holds a slash, so the key can
// not collide with a configuration of this instance.
func DelegatedFilename(nodeID uint64, name string) string {
	return fmt.Sprintf("node:%d/%s", nodeID, name)
}

// DelegatedCertificateDir is where this instance keeps a certificate issued
// for a node.
func DelegatedCertificateDir(nodeID uint64, serverName []string, keyType certcrypto.KeyType) string {
	return nginx.GetConfPath("ssl", delegatedDirName, fmt.Sprint(nodeID),
		strings.Join(serverName, "_")+"_"+string(helper.GetKeyType(keyType)))
}

// SyncPathsRequest asks a node where it keeps the certificate of one of its
// configurations.
type SyncPathsRequest struct {
	Name       string             `json:"name" binding:"required"`
	ServerName []string           `json:"server_name" binding:"required"`
	KeyType    certcrypto.KeyType `json:"key_type"`
}

// SyncPaths are the certificate files of a configuration on a node.
type SyncPaths struct {
	SSLCertificatePath    string `json:"ssl_certificate_path"`
	SSLCertificateKeyPath string `json:"ssl_certificate_key_path"`
}

// IssueForNode issues a DNS-01 certificate for a configuration of a node with
// the DNS providers of this instance, keeps it here for renewal and sends it
// to the node. The node is asked first where the files belong, since its
// Nginx configuration directory can differ from the one of this instance.
func IssueForNode(ctx context.Context, nodeID uint64, name string, payload *ConfigPayload, log *Logger) (*DelegatedIssue, error) {
	if payload.ChallengeMethod != DNS01 {
		return nil, ErrDelegationRequiresDNS01
	}
	node, err := query.Node.FirstByID(nodeID)
	if err != nil {
		return nil, ErrDelegationNodeNotFound
	}

	// A renewal keeps the files the configuration of the node already loads.
	remote := existingRemotePaths(node.ID, name, payload.GetKeyType())
	if remote == nil {
		if remote, err = RequestSyncPaths(ctx, node, SyncPathsRequest{
			Name: name, ServerName: payload.ServerName, KeyType: payload.GetKeyType(),
		}); err != nil {
			return nil, err
		}
	}

	payload.CertificateDir = DelegatedCertificateDir(node.ID, payload.ServerName, payload.GetKeyType())
	certModel, issueErr := IssueWithRecord(DelegatedFilename(node.ID, name), payload, log)
	if certModel == nil {
		return nil, issueErr
	}
	issued := &DelegatedIssue{Cert: certModel}

	certModel.DelegatedNodeID = node.ID
	certModel.DelegatedConfigName = name
	certModel.RemoteSSLCertificatePath = remote.SSLCertificatePath
	certModel.RemoteSSLCertificateKeyPath = remote.SSLCertificateKeyPath
	if err = model.UseDB().Model(&model.Cert{}).Where("id = ?", certModel.ID).Updates(map[string]any{
		"delegated_node_id":               node.ID,
		"delegated_config_name":           name,
		"remote_ssl_certificate_path":     remote.SSLCertificatePath,
		"remote_ssl_certificate_key_path": remote.SSLCertificateKeyPath,
	}).Error; err != nil {
		return issued, cosy.WrapErrorWithParams(ErrPersistCertificateRecord, err.Error())
	}
	if issueErr != nil {
		return issued, issueErr
	}

	// The record holds the paths of the files that were just written.
	if fresh, err := query.Cert.FirstByID(certModel.ID); err == nil {
		issued.Cert = fresh
	}
	log.Info(translation.C("[Nginx UI] Sending the certificate to node %{name}", map[string]any{"name": node.Name}))
	if issued.RemoteCertificateID, err = pushDelegated(node, issued.Cert); err != nil {
		return issued, cosy.WrapErrorWithParams(ErrPushCertificateToNode, node.Name, err.Error())
	}
	return issued, nil
}

// DelegatedIssue is a certificate issued for a node.
type DelegatedIssue struct {
	Cert *model.Cert
	// RemoteCertificateID is the record the node keeps the certificate in,
	// 0 when the node did not name it.
	RemoteCertificateID uint64
}

// existingRemotePaths returns the paths on the node of a certificate this
// instance already issued for the configuration, or nil.
func existingRemotePaths(nodeID uint64, name string, keyType certcrypto.KeyType) *SyncPaths {
	db := model.UseDB()
	if db == nil {
		return nil
	}
	var existing model.Cert
	if err := db.Where("filename = ? AND key_type IN ?", DelegatedFilename(nodeID, name),
		helper.GetKeyTypeAliasStrings(helper.GetKeyType(keyType))).First(&existing).Error; err != nil {
		return nil
	}
	if existing.RemoteSSLCertificatePath == "" || existing.RemoteSSLCertificateKeyPath == "" {
		return nil
	}
	return &SyncPaths{
		SSLCertificatePath:    existing.RemoteSSLCertificatePath,
		SSLCertificateKeyPath: existing.RemoteSSLCertificateKeyPath,
	}
}

// pushDelegated sends a certificate issued for a node to its paths there and
// returns the record the node keeps it in.
func pushDelegated(node *model.Node, c *model.Cert) (uint64, error) {
	payload, err := newSyncPayload(c.Name, c.SSLCertificatePath, c.SSLCertificateKeyPath, c.GetKeyType())
	if err != nil {
		return 0, err
	}
	payload.SSLCertificatePath = c.RemoteSSLCertificatePath
	payload.SSLCertificateKeyPath = c.RemoteSSLCertificateKeyPath
	payload.Delegated = true
	body, err := json.Marshal(payload)
	if err != nil {
		return 0, err
	}
	reply, err := deployWithReply(node, c, body)
	if err != nil {
		return 0, err
	}
	var record struct {
		ID uint64 `json:"id"`
	}
	_ = json.Unmarshal(reply, &record)
	return record.ID, nil
}

// syncDelegated sends a renewed certificate to the node it was issued for.
func syncDelegated(c *model.Cert) error {
	node, err := query.Node.FirstByID(c.DelegatedNodeID)
	if err != nil {
		return ErrDelegationNodeNotFound
	}
	_, err = pushDelegated(node, c)
	return err
}

// RequestSyncPaths asks a node where the certificate of a configuration goes.
func RequestSyncPaths(ctx context.Context, node *model.Node, request SyncPathsRequest) (*SyncPaths, error) {
	var paths SyncPaths
	if err := nodeRequest(ctx, node, http.MethodPost, "/api/cert_sync/paths", request, &paths); err != nil {
		return nil, err
	}
	if paths.SSLCertificatePath == "" || paths.SSLCertificateKeyPath == "" {
		return nil, cosy.WrapErrorWithParams(ErrNodeCannotReceiveCertificates, node.Name)
	}
	return &paths, nil
}

// RemoveDelegated deletes a certificate issued for a node. Its files on the
// node stay unless removeRemote is set, since a configuration there may still
// load them.
func RemoveDelegated(ctx context.Context, c *model.Cert, removeRemote bool) error {
	if removeRemote {
		node, err := query.Node.FirstByID(c.DelegatedNodeID)
		if err != nil {
			return ErrDelegationNodeNotFound
		}
		if err = nodeRequest(ctx, node, http.MethodDelete, "/api/cert_sync", SyncPaths{
			SSLCertificatePath:    c.RemoteSSLCertificatePath,
			SSLCertificateKeyPath: c.RemoteSSLCertificateKeyPath,
		}, nil); err != nil {
			return err
		}
	}

	delegatedRoot := nginx.GetConfPath("ssl", delegatedDirName)
	for _, path := range []string{c.SSLCertificatePath, c.SSLCertificateKeyPath} {
		if path != "" && helper.IsUnderDirectory(path, delegatedRoot) {
			_ = nginx.Remove(path)
		}
	}
	if dir := filepath.Dir(c.SSLCertificatePath); c.SSLCertificatePath != "" && helper.IsUnderDirectory(dir, delegatedRoot) {
		_ = nginx.Remove(dir)
	}
	_, err := query.Cert.Where(query.Cert.ID.Eq(c.ID)).Delete()
	return err
}

// nodeRequest sends a signed JSON request to a node. A node that does not
// know the endpoint is reported as one that cannot receive certificates.
func nodeRequest(ctx context.Context, node *model.Node, method, uri string, body, out any) error {
	client, err := nodeauth.NewHTTPClient(node, nodeRequestTimeout)
	if err != nil {
		return err
	}
	url, err := node.GetUrl(uri)
	if err != nil {
		return err
	}
	encoded, err := json.Marshal(body)
	if err != nil {
		return err
	}
	request, err := http.NewRequestWithContext(ctx, method, url, bytes.NewReader(encoded))
	if err != nil {
		return err
	}
	request.Header.Set("Content-Type", "application/json")

	response, err := client.Do(request)
	if err != nil {
		return cosy.WrapErrorWithParams(ErrNodeUnreachable, node.Name, err.Error())
	}
	defer response.Body.Close()
	data, _ := io.ReadAll(io.LimitReader(response.Body, 1<<20))

	if response.StatusCode == http.StatusNotFound || response.StatusCode == http.StatusMethodNotAllowed {
		return cosy.WrapErrorWithParams(ErrNodeCannotReceiveCertificates, node.Name)
	}
	if response.StatusCode < 200 || response.StatusCode >= 300 {
		var reply struct {
			Message string `json:"message"`
		}
		message := strings.TrimSpace(string(data))
		if json.Unmarshal(data, &reply) == nil && reply.Message != "" {
			message = reply.Message
		}
		return cosy.WrapErrorWithParams(ErrNodeRequestFailed, node.Name, response.Status, message)
	}
	if out == nil {
		return nil
	}
	if err = json.Unmarshal(data, out); err != nil {
		return cosy.WrapErrorWithParams(ErrNodeCannotReceiveCertificates, node.Name)
	}
	return nil
}
