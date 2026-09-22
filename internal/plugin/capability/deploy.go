package capability

import (
	"context"
	"time"

	"github.com/0xJacky/Nginx-UI/internal/cert/deploy"
	"github.com/0xJacky/Nginx-UI/internal/plugin"
	"github.com/0xJacky/Nginx-UI/internal/plugin/jsonrpc"
	"github.com/0xJacky/Nginx-UI/internal/plugin/protocol"
	"github.com/uozi-tech/cosy"
	"github.com/uozi-tech/cosy/logger"
)

const (
	// deployPushTimeout bounds one push, including starting an on_demand
	// plugin.
	deployPushTimeout = 5 * time.Minute
	// deployValidateTimeout bounds a configuration check, which never
	// contacts the target.
	deployValidateTimeout = 30 * time.Second
)

// DeployHost is the part of the plugin manager the cert.deploy capability
// needs.
type DeployHost interface {
	// OwnerOf returns the plugin that serves a target kind code. Only a
	// plugin granted the cert.deploy permission owns one.
	OwnerOf(capability, code string) (pluginID string, ok bool)
	// Acquire returns a client for a plugin and a func that releases it,
	// starting an on_demand plugin. The release func is safe to call even
	// when the error is not nil.
	Acquire(ctx context.Context, pluginID string) (jsonrpc.Caller, func(), error)
	// DeployTargets lists what every enabled cert.deploy plugin offers.
	DeployTargets() []plugin.DeployTargetEntry
}

// RegisterDeploy offers the target kinds of every enabled cert.deploy plugin
// to the certificate deploy targets.
func RegisterDeploy(h DeployHost) {
	deploy.RegisterSource(NewDeploySource(h))
}

// NewDeploySource exposes the cert.deploy capability of the plugins of h as
// a target kind source. A kind code is published as PluginType(code).
func NewDeploySource(h DeployHost) deploy.Source {
	return &deploySource{host: h}
}

type deploySource struct {
	host DeployHost
}

// Kinds lists every target kind once, served by the plugin that owns its
// code.
func (s *deploySource) Kinds() []deploy.TargetKind {
	entries := s.host.DeployTargets()
	kinds := make([]deploy.TargetKind, 0, len(entries))
	for _, entry := range entries {
		if owner, ok := s.host.OwnerOf(protocol.CapabilityCertDeploy, entry.Target.Code); !ok || owner != entry.PluginID {
			continue
		}
		fields := configurationFields(entry.Target.Configuration)
		kind := deploy.TargetKind{
			Kind:     PluginType(entry.Target.Code),
			Name:     entry.Target.Name,
			PluginID: entry.PluginID,
			Fields:   make([]deploy.KindField, 0, len(fields)),
		}
		for _, field := range fields {
			kind.Fields = append(kind.Fields, deploy.KindField{
				Key:         field.Key,
				Type:        field.Type,
				DisplayName: field.DisplayName,
				HelpText:    field.HelpText,
				Required:    field.Required,
				Secret:      field.Secret,
			})
		}
		kinds = append(kinds, kind)
	}
	return kinds
}

// owner resolves the plugin that serves a kind right now.
func (s *deploySource) owner(kind string) (pluginID, code string, err error) {
	code, ok := pluginCode(kind)
	if !ok {
		return "", "", cosy.WrapErrorWithParams(plugin.ErrDeployKindUnavailable, kind)
	}
	pluginID, ok = s.host.OwnerOf(protocol.CapabilityCertDeploy, code)
	if !ok {
		return "", "", cosy.WrapErrorWithParams(plugin.ErrDeployKindUnavailable, kind)
	}
	return pluginID, code, nil
}

// Validate asks the owning plugin to check a configuration. Only a
// CodeInvalidConfig answer rejects it: a plugin that does not implement
// deploy.validate or cannot be reached right now has no opinion, and the
// next push reports the problem.
func (s *deploySource) Validate(ctx context.Context, kind string, config map[string]string) error {
	pluginID, code, err := s.owner(kind)
	if err != nil {
		return err
	}

	callCtx, cancel := context.WithTimeout(ctx, deployValidateTimeout)
	defer cancel()

	caller, release, err := s.host.Acquire(callCtx, pluginID)
	if release != nil {
		defer release()
	}
	if err != nil {
		logger.Warnf("[plugin:%s] cannot validate deploy target kind %s: %v", pluginID, code, err)
		return nil
	}

	err = caller.Call(callCtx, protocol.MethodDeployValidate, protocol.DeployValidateParams{
		Kind:   code,
		Config: config,
	}, nil)
	switch {
	case err == nil, isUnimplemented(err):
		return nil
	}
	if field, message, invalid := invalidConfigField(err); invalid {
		return cosy.WrapErrorWithParams(plugin.ErrDeployConfigInvalid, field, message)
	}
	logger.Warnf("[plugin:%s] validate deploy target kind %s: %s", pluginID, code, rpcMessage(err))
	return nil
}

// Push runs deploy.push on the plugin that owns the kind.
func (s *deploySource) Push(ctx context.Context, kind string, config map[string]string, cert deploy.Certificate, dryRun bool) (string, error) {
	pluginID, code, err := s.owner(kind)
	if err != nil {
		return "", err
	}

	callCtx, cancel := context.WithTimeout(ctx, deployPushTimeout)
	defer cancel()

	caller, release, err := s.host.Acquire(callCtx, pluginID)
	if release != nil {
		defer release()
	}
	if err != nil {
		return "", plugin.WrapRPCError(err)
	}

	params := protocol.DeployPushParams{
		Kind:   code,
		Config: config,
		Certificate: protocol.DeployCertificate{
			Name:           cert.Name,
			Domains:        cert.Domains,
			CertificatePEM: cert.CertificatePEM,
			PrivateKeyPEM:  cert.PrivateKeyPEM,
			ChainPEM:       cert.ChainPEM,
		},
		DryRun: dryRun,
	}
	if !cert.NotAfter.IsZero() {
		params.Certificate.NotAfter = cert.NotAfter.UTC().Format(time.RFC3339)
	}

	var result protocol.DeployPushResult
	if err = caller.Call(callCtx, protocol.MethodDeployPush, params, &result); err != nil {
		if field, message, invalid := invalidConfigField(err); invalid {
			return "", cosy.WrapErrorWithParams(plugin.ErrDeployConfigInvalid, field, message)
		}
		return "", plugin.WrapRPCError(err)
	}
	return result.Message, nil
}
