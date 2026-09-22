package protocol

// Lifecycle methods (host -> plugin).
const (
	MethodInitialize  = "plugin.initialize"  // request
	MethodInitialized = "plugin.initialized" // notification
	MethodConfigure   = "plugin.configure"   // request
	MethodPing        = "plugin.ping"        // request
	MethodShutdown    = "plugin.shutdown"    // request
	MethodExit        = "plugin.exit"        // notification
)

// Capability dns01 methods (host -> plugin).
const (
	MethodDNS01Present  = "dns01.present"
	MethodDNS01Cleanup  = "dns01.cleanup"
	MethodDNS01Options  = "dns01.options"
	MethodDNS01Check    = "dns01.check"
	MethodDNS01Validate = "dns01.validate"
)

// Capability http methods (host -> plugin).
const (
	MethodHTTPHandle = "http.handle"
)

// Capability notify methods (host -> plugin).
const (
	MethodNotifySend     = "notify.send"
	MethodNotifyValidate = "notify.validate"
)

// Capability probe methods (host -> plugin).
const (
	MethodProbeCheck = "probe.check"
)

// Capability mcp methods (host -> plugin).
const (
	MethodMCPCall = "mcp.call"
)

// Capability storage methods (host -> plugin).
const (
	MethodStorageValidate = "storage.validate"
	MethodStoragePut      = "storage.put"
	MethodStorageGet      = "storage.get"
	MethodStorageList     = "storage.list"
	MethodStorageDelete   = "storage.delete"
)

// Capability cert.deploy methods (host -> plugin).
const (
	MethodDeployValidate = "deploy.validate"
	MethodDeployPush     = "deploy.push"
)

// Event and cron delivery (host -> plugin).
const (
	MethodEventsOn = "events.on" // notification
)

// Host API methods (plugin -> host).
const (
	MethodHostLog             = "host.log"
	MethodHostKVGet           = "host.kv.get"
	MethodHostKVSet           = "host.kv.set"
	MethodHostKVDelete        = "host.kv.delete"
	MethodHostKVList          = "host.kv.list"
	MethodHostSettingsGet     = "host.settings.get"
	MethodHostI18nLocale      = "host.i18n.locale"
	MethodHostCredentialsGet  = "host.credentials.get"
	MethodHostCronRegister    = "host.cron.register"
	MethodHostCronUnregister  = "host.cron.unregister"
	MethodHostNotify          = "host.notify"
	MethodHostMetricsSnapshot = "host.metrics.snapshot"
)

// Capability names a plugin may declare in its manifest.
const (
	CapabilityDNS01  = "dns01"
	CapabilityHTTP   = "http"
	CapabilityNotify = "notify"
	CapabilityProbe  = "probe"
	CapabilityMCP    = "mcp"
	// CapabilityStorage keeps host files in the backends of the manifest.
	CapabilityStorage = "storage"
	// CapabilityCertDeploy pushes certificates to external targets.
	CapabilityCertDeploy = "cert.deploy"
)

// Permission names a plugin may request in its manifest.
const (
	PermissionKV          = "kv"
	PermissionNetwork     = "network"
	PermissionCron        = "cron"
	PermissionNotify      = "notify"
	PermissionMetricsRead = "metrics.read"
	PermissionCoreAPI     = "core_api"
	// PermissionMCP lets the host publish the tools of the mcp capability.
	PermissionMCP = "mcp"
	// PermissionCertDeploy lets the host send certificates and their private
	// keys to the cert.deploy capability.
	PermissionCertDeploy = "cert.deploy"
	// PermissionCredentialsReadPrefix is followed by the credential kind, e.g. "credentials.read:dns".
	PermissionCredentialsReadPrefix = "credentials.read:"
)

// Transport names a plugin may advertise in InitializeResult.Transports.
const (
	TransportStdio = "stdio"
	TransportGRPC  = "grpc"
)

// Lifecycle values for Manifest.Server.Lifecycle.
const (
	LifecycleResident = "resident"
	LifecycleOnDemand = "on_demand"
)
