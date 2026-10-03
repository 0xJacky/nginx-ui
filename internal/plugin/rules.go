package plugin

// Rules name the checks of the plugin linter and the conformance runner.
// Every rule has a section in the plugin development guide, see RulesURL.
const (
	// Manifest
	RuleManifestJSON        = "manifest-json"
	RuleManifestID          = "manifest-id"
	RuleManifestName        = "manifest-name"
	RuleManifestVersion     = "manifest-version"
	RuleManifestAPIVersion  = "manifest-api-version"
	RuleManifestParts       = "manifest-parts"
	RuleManifestIcon        = "manifest-icon"
	RuleManifestI18n        = "manifest-i18n"
	RuleManifestRequires    = "manifest-requires"
	RuleManifestConflicts   = "manifest-conflicts"
	RuleManifestPermissions = "manifest-permissions"
	RuleManifestReasons     = "manifest-permission-reasons"
	RuleManifestScreenshots = "manifest-screenshots"
	RuleCapabilityName      = "capability-name"
	RuleCapabilityDuplicate = "capability-duplicate"
	RuleServerEntry         = "server-entry"
	RuleServerCommand       = "server-command"
	RuleServerLifecycle     = "server-lifecycle"
	RuleServerIdleTimeout   = "server-idle-timeout"
	RuleServerPaths         = "server-paths"
	RuleServerResources     = "server-resources"
	RuleSettingsKey         = "settings-key"
	RuleSettingsType        = "settings-type"
	RuleSettingsOptions     = "settings-options"
	RuleEventPermission     = "event-permission"
	RuleReservedNamespace   = "reserved-namespace"
	RuleNetworkPermission   = "network-permission"
	RuleUnusedPermission    = "unused-permission"

	// Package
	RulePackageFileName    = "package-file-name"
	RulePackageLayout      = "package-layout"
	RulePackagePaths       = "package-paths"
	RulePackageLinks       = "package-links"
	RulePackageEscape      = "package-escape"
	RulePackageEntries     = "package-entries"
	RulePackageSize        = "package-size"
	RulePackageDocs        = "package-docs"
	RulePackageExecutables = "package-executables"
	RulePackagePlatform    = "package-platform"

	// Signature
	RuleSignatureSums      = "signature-sums"
	RuleSignatureFiles     = "signature-files"
	RuleSignatureMismatch  = "signature-mismatch"
	RuleSignatureSigner    = "signature-signer"
	RulePartnerFiles       = "partner-files"
	RulePartnerComment     = "partner-comment"
	RulePartnerCertificate = "partner-certificate"

	// Protocol and lifecycle
	RuleProtocolStderr        = "protocol-stderr"
	RuleProtocolNotification  = "protocol-notification"
	RuleProtocolConcurrency   = "protocol-concurrency"
	RuleProtocolErrors        = "protocol-errors"
	RuleProtocolGRPC          = "protocol-grpc"
	RuleProtocolTransports    = "protocol-transports"
	RuleLifecycleHandshake    = "lifecycle-handshake"
	RuleLifecycleAPIVersion   = "lifecycle-api-version"
	RuleLifecycleCapabilities = "lifecycle-capabilities"
	RuleLifecyclePing         = "lifecycle-ping"
	RuleLifecycleShutdown     = "lifecycle-shutdown"

	// Web interface
	RuleWebappPaths      = "webapp-paths"
	RuleWebappPages      = "webapp-pages"
	RuleWebappChunks     = "webapp-chunks"
	RuleWebappBundle     = "webapp-bundle"
	RuleWebappRegister   = "webapp-register"
	RuleWebappChunkFiles = "webapp-chunk-files"

	// Capabilities
	RuleDNS01Block          = "dns01-block"
	RuleDNS01Code           = "dns01-code"
	RuleDNS01Name           = "dns01-name"
	RuleDNS01Form           = "dns01-form"
	RuleDNS01Present        = "dns01-present"
	RuleDNS01Validate       = "dns01-validate"
	RuleDNS01Options        = "dns01-options"
	RuleDNS01Check          = "dns01-check"
	RuleHTTPBlock           = "http-block"
	RuleConfigurationFields = "configuration-fields"
	RuleNotifyBlock         = "notify-block"
	RuleNotifyCode          = "notify-code"
	RuleNotifyName          = "notify-name"
	RuleNotifyValidate      = "notify-validate"
	RuleProbeBlock          = "probe-block"
	RuleProbeCode           = "probe-code"
	RuleProbeKind           = "probe-kind"
	RuleProbeCheck          = "probe-check"
	RuleProbeResult         = "probe-result"
	RuleMCPBlock            = "mcp-block"
	RuleMCPTool             = "mcp-tool"
	RuleMCPInputSchema      = "mcp-input-schema"
	RuleMCPUnknownTool      = "mcp-unknown-tool"
	RuleStorageBlock        = "storage-block"
	RuleStorageCode         = "storage-code"
	RuleStorageBackend      = "storage-backend"
	RuleStorageList         = "storage-list"
	RuleStorageValidate     = "storage-validate"
	RuleDeployBlock         = "deploy-block"
	RuleDeployCode          = "deploy-code"
	RuleDeployTarget        = "deploy-target"
	RuleDeployDryRun        = "deploy-dry-run"
	RuleDeployValidate      = "deploy-validate"
	RuleBlocklistBlock      = "blocklist-block"
	RuleBlocklistCode       = "blocklist-code"
	RuleBlocklistSource     = "blocklist-source"
	RuleBlocklistFetch      = "blocklist-fetch"
	RuleBlocklistErrors     = "blocklist-errors"
	RuleDiscoveryBlock      = "discovery-block"
	RuleDiscoveryCode       = "discovery-code"
	RuleDiscoveryProvider   = "discovery-provider"
	RuleDiscoveryResolve    = "discovery-resolve"
	RuleDiscoveryErrors     = "discovery-errors"
	RuleLogSinkPermission   = "log-sink-permission"
	RuleLogSinkBlock        = "log-sink-block"
	RuleLogSinkBatch        = "log-sink-batch"
	RuleLogSinkFormats      = "log-sink-formats"
	RuleLogSinkTransport    = "log-sink-transport"
	RuleLogSinkPush         = "log-sink-push"

	// Content
	RuleContentPaths       = "content-paths"
	RuleContentProcessLess = "content-without-server"
	RuleContentTemplates   = "content-templates"
	RuleContentTemplate    = "content-template"
	RuleContentLocales     = "content-locales"
	RuleContentLocale      = "content-locale"
)

// RulesURL is the page that explains every rule, one section per rule name.
const RulesURL = "https://nginxui.com/plugin/rules"

// AllRules lists every rule name.
var AllRules = []string{
	RuleManifestJSON,
	RuleManifestID,
	RuleManifestName,
	RuleManifestVersion,
	RuleManifestAPIVersion,
	RuleManifestParts,
	RuleManifestIcon,
	RuleManifestI18n,
	RuleManifestRequires,
	RuleManifestConflicts,
	RuleManifestPermissions,
	RuleManifestReasons,
	RuleManifestScreenshots,
	RuleCapabilityName,
	RuleCapabilityDuplicate,
	RuleServerEntry,
	RuleServerCommand,
	RuleServerLifecycle,
	RuleServerIdleTimeout,
	RuleServerPaths,
	RuleServerResources,
	RuleSettingsKey,
	RuleSettingsType,
	RuleSettingsOptions,
	RuleEventPermission,
	RuleReservedNamespace,
	RuleNetworkPermission,
	RuleUnusedPermission,
	RulePackageFileName,
	RulePackageLayout,
	RulePackagePaths,
	RulePackageLinks,
	RulePackageEscape,
	RulePackageEntries,
	RulePackageSize,
	RulePackageDocs,
	RulePackageExecutables,
	RulePackagePlatform,
	RuleSignatureSums,
	RuleSignatureFiles,
	RuleSignatureMismatch,
	RuleSignatureSigner,
	RulePartnerFiles,
	RulePartnerComment,
	RulePartnerCertificate,
	RuleProtocolStderr,
	RuleProtocolNotification,
	RuleProtocolConcurrency,
	RuleProtocolErrors,
	RuleProtocolGRPC,
	RuleProtocolTransports,
	RuleLifecycleHandshake,
	RuleLifecycleAPIVersion,
	RuleLifecycleCapabilities,
	RuleLifecyclePing,
	RuleLifecycleShutdown,
	RuleWebappPaths,
	RuleWebappPages,
	RuleWebappChunks,
	RuleWebappBundle,
	RuleWebappRegister,
	RuleWebappChunkFiles,
	RuleDNS01Block,
	RuleDNS01Code,
	RuleDNS01Name,
	RuleDNS01Form,
	RuleDNS01Present,
	RuleDNS01Validate,
	RuleDNS01Options,
	RuleDNS01Check,
	RuleHTTPBlock,
	RuleConfigurationFields,
	RuleNotifyBlock,
	RuleNotifyCode,
	RuleNotifyName,
	RuleNotifyValidate,
	RuleProbeBlock,
	RuleProbeCode,
	RuleProbeKind,
	RuleProbeCheck,
	RuleProbeResult,
	RuleMCPBlock,
	RuleMCPTool,
	RuleMCPInputSchema,
	RuleMCPUnknownTool,
	RuleStorageBlock,
	RuleStorageCode,
	RuleStorageBackend,
	RuleStorageList,
	RuleStorageValidate,
	RuleDeployBlock,
	RuleDeployCode,
	RuleDeployTarget,
	RuleDeployDryRun,
	RuleDeployValidate,
	RuleBlocklistBlock,
	RuleBlocklistCode,
	RuleBlocklistSource,
	RuleBlocklistFetch,
	RuleBlocklistErrors,
	RuleDiscoveryBlock,
	RuleDiscoveryCode,
	RuleDiscoveryProvider,
	RuleDiscoveryResolve,
	RuleDiscoveryErrors,
	RuleLogSinkPermission,
	RuleLogSinkBlock,
	RuleLogSinkBatch,
	RuleLogSinkFormats,
	RuleLogSinkTransport,
	RuleLogSinkPush,
	RuleContentPaths,
	RuleContentProcessLess,
	RuleContentTemplates,
	RuleContentTemplate,
	RuleContentLocales,
	RuleContentLocale,
}
