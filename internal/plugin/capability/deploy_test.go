package capability

import (
	"encoding/json"
	"errors"
	"testing"
	"time"

	"github.com/0xJacky/Nginx-UI/internal/cert/deploy"
	"github.com/0xJacky/Nginx-UI/internal/plugin"
	"github.com/0xJacky/Nginx-UI/internal/plugin/protocol"
)

const deployPluginID = "io.github.example.cdn"

func deployHost(caller *fakeCaller) *capabilityHost {
	h := newCapabilityHost()
	h.callers[deployPluginID] = caller
	h.targets = []plugin.DeployTargetEntry{
		{PluginID: deployPluginID, Target: protocol.DeployTarget{
			Code: "mycdn",
			Name: "MyCDN",
			Configuration: &protocol.ConfigurationSchema{Fields: []protocol.ConfigurationField{
				{Key: "zone_id", DisplayName: "Zone ID", Required: true},
				{Key: "api_token", DisplayName: "API token", Secret: true},
			}},
		}},
		// A second plugin declaring the same code does not own it.
		{PluginID: "io.github.other.cdn", Target: protocol.DeployTarget{Code: "mycdn", Name: "Shadowed"}},
	}
	h.own(protocol.CapabilityCertDeploy, "mycdn", deployPluginID)
	return h
}

func testDeployCertificate() deploy.Certificate {
	return deploy.Certificate{
		Name:           "example.com",
		Domains:        []string{"example.com", "www.example.com"},
		CertificatePEM: "LEAF",
		PrivateKeyPEM:  "KEY",
		ChainPEM:       "ISSUER",
		NotAfter:       time.Date(2026, 12, 20, 8, 15, 0, 0, time.FixedZone("JST", 9*3600)),
	}
}

func TestDeployKindsListTheOwnedCodes(t *testing.T) {
	kinds := NewDeploySource(deployHost(newFakeCaller())).Kinds()
	if len(kinds) != 1 || kinds[0].Kind != "plugin:mycdn" || kinds[0].Name != "MyCDN" || kinds[0].PluginID != deployPluginID {
		t.Fatalf("kinds = %+v", kinds)
	}
	if len(kinds[0].Fields) != 2 || !kinds[0].Fields[0].Required || !kinds[0].Fields[1].Secret {
		t.Fatalf("fields = %+v", kinds[0].Fields)
	}
}

func TestDeployPushCallsThePlugin(t *testing.T) {
	caller := newFakeCaller()
	caller.results[protocol.MethodDeployPush] = protocol.DeployPushResult{Message: "bound to zone_123"}
	host := deployHost(caller)
	source := NewDeploySource(host)

	message, err := source.Push(t.Context(), "plugin:mycdn", map[string]string{"zone_id": "zone_123"}, testDeployCertificate(), true)
	if err != nil {
		t.Fatalf("push: %v", err)
	}
	if message != "bound to zone_123" {
		t.Fatalf("message = %q", message)
	}

	calls := caller.methodCalls(protocol.MethodDeployPush)
	if len(calls) != 1 {
		t.Fatalf("deploy.push calls = %d", len(calls))
	}
	var params protocol.DeployPushParams
	if err = json.Unmarshal(calls[0].Params, &params); err != nil {
		t.Fatal(err)
	}
	if params.Kind != "mycdn" || !params.DryRun || params.Config["zone_id"] != "zone_123" {
		t.Fatalf("params = %+v", params)
	}
	c := params.Certificate
	if c.Name != "example.com" || len(c.Domains) != 2 || c.CertificatePEM != "LEAF" || c.PrivateKeyPEM != "KEY" ||
		c.ChainPEM != "ISSUER" || c.NotAfter != "2026-12-19T23:15:00Z" {
		t.Fatalf("certificate = %+v", c)
	}
	if host.releaseCount() != 1 {
		t.Fatalf("releases = %d, want 1", host.releaseCount())
	}
}

func TestDeployPushMapsTheErrors(t *testing.T) {
	caller := newFakeCaller()
	source := NewDeploySource(deployHost(caller))

	caller.errs[protocol.MethodDeployPush] = &protocol.Error{
		Code: protocol.CodeInvalidConfig, Message: "api_token was rejected", Data: map[string]any{"field": "api_token"},
	}
	_, err := source.Push(t.Context(), "plugin:mycdn", nil, testDeployCertificate(), false)
	assertCosyCode(t, err, plugin.ErrDeployConfigInvalid)

	caller.errs[protocol.MethodDeployPush] = &protocol.Error{Code: protocol.CodeInternalError, Message: "cdn is down"}
	_, err = source.Push(t.Context(), "plugin:mycdn", nil, testDeployCertificate(), false)
	if !errors.Is(err, plugin.ErrRPC) {
		t.Fatalf("err = %v, want the plugin rpc error", err)
	}

	for _, kind := range []string{"plugin:unknown", "mycdn"} {
		_, err = source.Push(t.Context(), kind, nil, testDeployCertificate(), false)
		assertCosyCode(t, err, plugin.ErrDeployKindUnavailable)
	}
}

func TestDeployValidate(t *testing.T) {
	caller := newFakeCaller()
	host := deployHost(caller)
	source := NewDeploySource(host)

	if err := source.Validate(t.Context(), "plugin:mycdn", map[string]string{"zone_id": "z"}); err != nil {
		t.Fatalf("validate: %v", err)
	}

	caller.errs[protocol.MethodDeployValidate] = &protocol.Error{
		Code: protocol.CodeInvalidConfig, Message: "zone_id is required", Data: map[string]any{"field": "zone_id"},
	}
	assertCosyCode(t, source.Validate(t.Context(), "plugin:mycdn", nil), plugin.ErrDeployConfigInvalid)

	// Unimplemented, failing or unreachable plugins have no opinion.
	for _, err := range []error{
		&protocol.Error{Code: protocol.CodeUnsupported, Message: "unsupported"},
		&protocol.Error{Code: protocol.CodeMethodNotFound, Message: "unknown"},
		&protocol.Error{Code: protocol.CodeInternalError, Message: "boom"},
	} {
		caller.errs[protocol.MethodDeployValidate] = err
		if got := source.Validate(t.Context(), "plugin:mycdn", nil); got != nil {
			t.Fatalf("validate with %v = %v, want nil", err, got)
		}
	}
	host.acquireErr = errors.New("cannot start")
	if err := source.Validate(t.Context(), "plugin:mycdn", nil); err != nil {
		t.Fatalf("validate of an unreachable plugin = %v, want nil", err)
	}

	// A kind nobody serves is rejected.
	assertCosyCode(t, source.Validate(t.Context(), "plugin:unknown", nil), plugin.ErrDeployKindUnavailable)
}

func TestDeploySourceThroughTheRegistry(t *testing.T) {
	caller := newFakeCaller()
	caller.results[protocol.MethodDeployPush] = protocol.DeployPushResult{Message: "ok"}
	deploy.RegisterSource(NewDeploySource(deployHost(caller)))

	var found bool
	for _, kind := range deploy.Kinds() {
		found = found || kind.Kind == "plugin:mycdn"
	}
	if !found {
		t.Fatal("the plugin kind is not offered")
	}

	// Required fields are checked before the plugin is asked.
	assertCosyCode(t, deploy.ValidateConfig(t.Context(), "plugin:mycdn", map[string]string{}), plugin.ErrDeployConfigInvalid)
	if len(caller.methodCalls(protocol.MethodDeployValidate)) != 0 {
		t.Fatal("the plugin was asked although a required field is empty")
	}
	if err := deploy.ValidateConfig(t.Context(), "plugin:mycdn", map[string]string{"zone_id": "z"}); err != nil {
		t.Fatalf("validate: %v", err)
	}

	message, err := deploy.Push(t.Context(), "plugin:mycdn", map[string]string{"zone_id": "z"}, testDeployCertificate(), false)
	if err != nil || message != "ok" {
		t.Fatalf("push = %q, %v", message, err)
	}
}
