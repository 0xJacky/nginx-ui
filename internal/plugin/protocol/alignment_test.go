package protocol_test

import (
	"reflect"
	"slices"
	"sort"
	"strings"
	"testing"

	"github.com/0xJacky/Nginx-UI/internal/plugin/protocol"
	pluginv1 "github.com/0xJacky/Nginx-UI/internal/plugin/protocol/pb"
	"google.golang.org/protobuf/proto"
	"google.golang.org/protobuf/reflect/protoreflect"
	"google.golang.org/protobuf/reflect/protoregistry"
)

// contractPackage is the proto package of the plugin contract.
const contractPackage protoreflect.FullName = "nginxui.plugin.v1"

// alignment pairs a hand-written wire type with the proto message it mirrors.
type alignment struct {
	goType  reflect.Type
	message protoreflect.FullName
}

// alignments lists every request, result and nested type of this package.
// Messages without fields are covered by EmptyResult implicitly.
var alignments = []alignment{
	// lifecycle.go
	{reflect.TypeFor[protocol.HostInfo](), "HostInfo"},
	{reflect.TypeFor[protocol.InitializeParams](), "PluginInitializeRequest"},
	{reflect.TypeFor[protocol.InitializeResult](), "PluginInitializeResponse"},
	{reflect.TypeFor[protocol.ConfigureParams](), "PluginConfigureRequest"},

	// host.go
	{reflect.TypeFor[protocol.HostLogParams](), "HostLogRequest"},
	{reflect.TypeFor[protocol.HostKVGetParams](), "HostKVGetRequest"},
	{reflect.TypeFor[protocol.HostKVGetResult](), "HostKVGetResponse"},
	{reflect.TypeFor[protocol.HostKVSetParams](), "HostKVSetRequest"},
	{reflect.TypeFor[protocol.HostKVGetParams](), "HostKVDeleteRequest"},
	{reflect.TypeFor[protocol.HostKVListParams](), "HostKVListRequest"},
	{reflect.TypeFor[protocol.HostKVListResult](), "HostKVListResponse"},
	{reflect.TypeFor[protocol.HostSettingsGetResult](), "HostSettingsGetResponse"},
	{reflect.TypeFor[protocol.HostI18nLocaleResult](), "HostI18nLocaleResponse"},
	{reflect.TypeFor[protocol.HostCredentialsGetParams](), "HostCredentialsGetRequest"},
	{reflect.TypeFor[protocol.HostCredentialsGetResult](), "HostCredentialsGetResponse"},
	{reflect.TypeFor[protocol.HostCronRegisterParams](), "HostCronRegisterRequest"},
	{reflect.TypeFor[protocol.HostCronUnregisterParams](), "HostCronUnregisterRequest"},
	{reflect.TypeFor[protocol.HostNotifyParams](), "HostNotifyRequest"},
	{reflect.TypeFor[protocol.HostMetricsSnapshotResult](), "HostMetricsSnapshotResponse"},

	// dns01.go
	{reflect.TypeFor[protocol.DNS01ChallengeParams](), "DNS01PresentRequest"},
	{reflect.TypeFor[protocol.DNS01ChallengeParams](), "DNS01CleanupRequest"},
	{reflect.TypeFor[protocol.DNS01OptionsParams](), "DNS01OptionsRequest"},
	{reflect.TypeFor[protocol.DNS01OptionsResult](), "DNS01OptionsResponse"},
	{reflect.TypeFor[protocol.DNS01CheckParams](), "DNS01CheckRequest"},
	{reflect.TypeFor[protocol.DNS01CheckResult](), "DNS01CheckResponse"},
	{reflect.TypeFor[protocol.DNS01ValidateParams](), "DNS01ValidateRequest"},

	// http.go
	{reflect.TypeFor[protocol.HTTPHandleParams](), "HTTPHandleRequest"},
	{reflect.TypeFor[protocol.HTTPUser](), "HTTPUser"},
	{reflect.TypeFor[protocol.HTTPHandleResult](), "HTTPHandleResponse"},

	// notify.go
	{reflect.TypeFor[protocol.NotifySendParams](), "NotifySendRequest"},
	{reflect.TypeFor[protocol.NotifyValidateParams](), "NotifyValidateRequest"},

	// probe.go
	{reflect.TypeFor[protocol.ProbeCheckParams](), "ProbeCheckRequest"},
	{reflect.TypeFor[protocol.ProbeCheckResult](), "ProbeCheckResponse"},

	// mcp.go
	{reflect.TypeFor[protocol.MCPCallParams](), "MCPCallRequest"},
	{reflect.TypeFor[protocol.MCPCallResult](), "MCPCallResponse"},
	{reflect.TypeFor[protocol.MCPContent](), "MCPContent"},

	// storage.go
	{reflect.TypeFor[protocol.StorageValidateParams](), "StorageValidateRequest"},
	{reflect.TypeFor[protocol.StoragePutParams](), "StoragePutRequest"},
	{reflect.TypeFor[protocol.StorageSizeResult](), "StoragePutResponse"},
	{reflect.TypeFor[protocol.StorageGetParams](), "StorageGetRequest"},
	{reflect.TypeFor[protocol.StorageSizeResult](), "StorageGetResponse"},
	{reflect.TypeFor[protocol.StorageListParams](), "StorageListRequest"},
	{reflect.TypeFor[protocol.StorageListResult](), "StorageListResponse"},
	{reflect.TypeFor[protocol.StorageObject](), "StorageObject"},
	{reflect.TypeFor[protocol.StorageDeleteParams](), "StorageDeleteRequest"},

	// deploy.go
	{reflect.TypeFor[protocol.DeployValidateParams](), "DeployValidateRequest"},
	{reflect.TypeFor[protocol.DeployPushParams](), "DeployPushRequest"},
	{reflect.TypeFor[protocol.DeployCertificate](), "DeployCertificate"},
	{reflect.TypeFor[protocol.DeployPushResult](), "DeployPushResponse"},

	// blocklist.go
	{reflect.TypeFor[protocol.BlocklistFetchParams](), "BlocklistFetchRequest"},
	{reflect.TypeFor[protocol.BlocklistFetchResult](), "BlocklistFetchResponse"},
	{reflect.TypeFor[protocol.BlocklistEntry](), "BlocklistEntry"},

	// discovery.go
	{reflect.TypeFor[protocol.DiscoveryResolveParams](), "DiscoveryResolveRequest"},
	{reflect.TypeFor[protocol.DiscoveryResolveResult](), "DiscoveryResolveResponse"},
	{reflect.TypeFor[protocol.DiscoveryTarget](), "DiscoveryTarget"},

	// logsink.go
	{reflect.TypeFor[protocol.LogSinkPushParams](), "LogSinkPushRequest"},
	{reflect.TypeFor[protocol.LogEntry](), "LogEntry"},
	{reflect.TypeFor[protocol.LogSinkPushResult](), "LogSinkPushResponse"},

	// events.go
	{reflect.TypeFor[protocol.EventNotification](), "EventsOnRequest"},

	// errors.go
	{reflect.TypeFor[protocol.Error](), "PluginError"},
	{reflect.TypeFor[protocol.InvalidConfigData](), "InvalidConfigData"},

	// manifest.go
	{reflect.TypeFor[protocol.Manifest](), "Manifest"},
	{reflect.TypeFor[protocol.ManifestServer](), "ManifestServer"},
	{reflect.TypeFor[protocol.ManifestResources](), "ManifestResources"},
	{reflect.TypeFor[protocol.ManifestWebapp](), "ManifestWebapp"},
	{reflect.TypeFor[protocol.ManifestPage](), "ManifestPage"},
	{reflect.TypeFor[protocol.ManifestContent](), "ManifestContent"},
	{reflect.TypeFor[protocol.ManifestRequirement](), "ManifestRequirement"},
	{reflect.TypeFor[protocol.ManifestCron](), "ManifestCron"},
	{reflect.TypeFor[protocol.ManifestDNS01](), "ManifestDNS01"},
	{reflect.TypeFor[protocol.DNS01Provider](), "DNS01Provider"},
	{reflect.TypeFor[protocol.DNS01ProviderConfig](), "DNS01ProviderConfig"},
	{reflect.TypeFor[protocol.DNS01ProviderLinks](), "DNS01ProviderLinks"},
	{reflect.TypeFor[protocol.ManifestHTTP](), "ManifestHTTP"},
	{reflect.TypeFor[protocol.SettingsSchema](), "SettingsSchema"},
	{reflect.TypeFor[protocol.SettingsField](), "SettingsField"},
	{reflect.TypeFor[protocol.SettingsOption](), "SettingsOption"},
	{reflect.TypeFor[protocol.ManifestNotify](), "ManifestNotify"},
	{reflect.TypeFor[protocol.NotifyChannel](), "NotifyChannel"},
	{reflect.TypeFor[protocol.ManifestProbe](), "ManifestProbe"},
	{reflect.TypeFor[protocol.ProbeKind](), "ProbeKind"},
	{reflect.TypeFor[protocol.ConfigurationSchema](), "ConfigurationSchema"},
	{reflect.TypeFor[protocol.ConfigurationField](), "ConfigurationField"},
	{reflect.TypeFor[protocol.ManifestMCP](), "ManifestMCP"},
	{reflect.TypeFor[protocol.MCPTool](), "MCPTool"},
	{reflect.TypeFor[protocol.ManifestStorage](), "ManifestStorage"},
	{reflect.TypeFor[protocol.StorageBackend](), "StorageBackend"},
	{reflect.TypeFor[protocol.ManifestDeploy](), "ManifestDeploy"},
	{reflect.TypeFor[protocol.DeployTarget](), "DeployTarget"},
	{reflect.TypeFor[protocol.ManifestBlocklist](), "ManifestBlocklist"},
	{reflect.TypeFor[protocol.BlocklistSource](), "BlocklistSource"},
	{reflect.TypeFor[protocol.ManifestDiscovery](), "ManifestDiscovery"},
	{reflect.TypeFor[protocol.DiscoveryProvider](), "DiscoveryProvider"},
	{reflect.TypeFor[protocol.ManifestLogSink](), "ManifestLogSink"},
}

// TestProtoAlignment asserts that the json tag names of every hand-written
// type equal the field names of its proto message and that each field has a
// compatible JSON shape.
func TestProtoAlignment(t *testing.T) {
	aligned := map[reflect.Type][]protoreflect.FullName{}
	for _, a := range alignments {
		aligned[a.goType] = append(aligned[a.goType], contractPackage.Append(protoreflect.Name(a.message)))
	}

	for _, a := range alignments {
		md := findMessage(t, a.message)
		tags := jsonFields(a.goType)

		var goNames, protoNames []string
		for name := range tags {
			goNames = append(goNames, name)
		}
		fields := md.Fields()
		for i := range fields.Len() {
			protoNames = append(protoNames, string(fields.Get(i).Name()))
		}

		for _, name := range missing(goNames, protoNames) {
			t.Errorf("%s: json tag %q has no field in %s", a.goType, name, md.FullName())
		}
		for _, name := range missing(protoNames, goNames) {
			t.Errorf("%s: field %s.%s has no json tag", a.goType, md.FullName(), name)
		}

		for i := range fields.Len() {
			fd := fields.Get(i)
			sf, ok := tags[string(fd.Name())]
			if !ok {
				continue
			}
			if !compatible(sf.Type, fd, aligned) {
				t.Errorf("%s.%s: Go type %s does not match proto field %s", a.goType, sf.Name, sf.Type, describe(fd))
			}
		}
	}
}

// TestEveryProtoMessageIsAligned asserts that no message of the contract is
// missing from the alignment table.
func TestEveryProtoMessageIsAligned(t *testing.T) {
	known := map[protoreflect.FullName]bool{}
	for _, a := range alignments {
		known[contractPackage.Append(protoreflect.Name(a.message))] = true
	}
	if n := len(jsonFields(reflect.TypeFor[protocol.EmptyResult]())); n != 0 {
		t.Fatalf("EmptyResult has %d fields", n)
	}

	for _, md := range contractMessages(t) {
		if md.Fields().Len() == 0 {
			continue
		}
		if !known[md.FullName()] {
			t.Errorf("proto message %s has no hand-written counterpart in the alignment table", md.FullName())
		}
	}
}

// TestMethodNamesMatchProto asserts that the method constants are exactly the
// rpc_name options of the contract, notifications and streams included.
func TestMethodNamesMatchProto(t *testing.T) {
	requests := []string{
		protocol.MethodInitialize,
		protocol.MethodConfigure,
		protocol.MethodPing,
		protocol.MethodShutdown,
		protocol.MethodDNS01Present,
		protocol.MethodDNS01Cleanup,
		protocol.MethodDNS01Options,
		protocol.MethodDNS01Check,
		protocol.MethodDNS01Validate,
		protocol.MethodHTTPHandle,
		protocol.MethodNotifySend,
		protocol.MethodNotifyValidate,
		protocol.MethodProbeCheck,
		protocol.MethodMCPCall,
		protocol.MethodStorageValidate,
		protocol.MethodStoragePut,
		protocol.MethodStorageGet,
		protocol.MethodStorageList,
		protocol.MethodStorageDelete,
		protocol.MethodDeployValidate,
		protocol.MethodDeployPush,
		protocol.MethodBlocklistFetch,
		protocol.MethodDiscoveryResolve,
		protocol.MethodHostLog,
		protocol.MethodHostKVGet,
		protocol.MethodHostKVSet,
		protocol.MethodHostKVDelete,
		protocol.MethodHostKVList,
		protocol.MethodHostSettingsGet,
		protocol.MethodHostI18nLocale,
		protocol.MethodHostCredentialsGet,
		protocol.MethodHostCronRegister,
		protocol.MethodHostCronUnregister,
		protocol.MethodHostNotify,
		protocol.MethodHostMetricsSnapshot,
	}
	notifications := []string{
		protocol.MethodInitialized,
		protocol.MethodExit,
		protocol.MethodEventsOn,
	}
	streams := []string{
		protocol.MethodLogPush,
	}

	var protoRequests, protoNotifications, protoStreams []string
	for _, file := range contractFiles(t) {
		services := file.Services()
		for i := range services.Len() {
			rpcs := services.Get(i).Methods()
			for j := range rpcs.Len() {
				md := rpcs.Get(j)
				name := proto.GetExtension(md.Options(), pluginv1.E_RpcName).(string)
				streaming := proto.GetExtension(md.Options(), pluginv1.E_Streaming).(bool)
				if streaming != md.IsStreamingClient() {
					t.Errorf("%s: the streaming option does not match the streamed request", name)
				}
				switch {
				case streaming:
					protoStreams = append(protoStreams, name)
				case proto.GetExtension(md.Options(), pluginv1.E_Notification).(bool):
					protoNotifications = append(protoNotifications, name)
				default:
					protoRequests = append(protoRequests, name)
				}
			}
		}
	}

	for _, name := range missing(requests, protoRequests) {
		t.Errorf("request %s is not an rpc of the proto", name)
	}
	for _, name := range missing(protoRequests, requests) {
		t.Errorf("rpc %s has no method constant", name)
	}
	for _, name := range missing(notifications, protoNotifications) {
		t.Errorf("notification %s is not a notification rpc of the proto", name)
	}
	for _, name := range missing(protoNotifications, notifications) {
		t.Errorf("notification rpc %s has no method constant", name)
	}
	for _, name := range missing(streams, protoStreams) {
		t.Errorf("stream %s is not a streaming rpc of the proto", name)
	}
	for _, name := range missing(protoStreams, streams) {
		t.Errorf("streaming rpc %s has no method constant", name)
	}
}

// TestErrorCodesMatchProto asserts that the error code constants equal the
// ErrorCode enum.
func TestErrorCodesMatchProto(t *testing.T) {
	codes := map[pluginv1.ErrorCode]int{
		pluginv1.ErrorCode_ERROR_CODE_PARSE_ERROR:       protocol.CodeParseError,
		pluginv1.ErrorCode_ERROR_CODE_INVALID_REQUEST:   protocol.CodeInvalidRequest,
		pluginv1.ErrorCode_ERROR_CODE_METHOD_NOT_FOUND:  protocol.CodeMethodNotFound,
		pluginv1.ErrorCode_ERROR_CODE_INVALID_PARAMS:    protocol.CodeInvalidParams,
		pluginv1.ErrorCode_ERROR_CODE_INTERNAL_ERROR:    protocol.CodeInternalError,
		pluginv1.ErrorCode_ERROR_CODE_PERMISSION_DENIED: protocol.CodePermissionDenied,
		pluginv1.ErrorCode_ERROR_CODE_UNSUPPORTED:       protocol.CodeUnsupported,
		pluginv1.ErrorCode_ERROR_CODE_INVALID_CONFIG:    protocol.CodeInvalidConfig,
	}

	values := pluginv1.ErrorCode(0).Descriptor().Values()
	for i := range values.Len() {
		value := pluginv1.ErrorCode(values.Get(i).Number())
		if value == pluginv1.ErrorCode_ERROR_CODE_UNSPECIFIED {
			continue
		}
		want, ok := codes[value]
		if !ok {
			t.Errorf("enum value %s has no error code constant", value)
			continue
		}
		if int(value) != want {
			t.Errorf("enum value %s is %d, constant is %d", value, int(value), want)
		}
	}
	if len(codes) != values.Len()-1 {
		t.Errorf("%d error code constants, %d enum values", len(codes), values.Len()-1)
	}
}

func findMessage(t *testing.T, name protoreflect.FullName) protoreflect.MessageDescriptor {
	t.Helper()

	d, err := protoregistry.GlobalFiles.FindDescriptorByName(contractPackage.Append(protoreflect.Name(name)))
	if err != nil {
		t.Fatalf("proto message %s: %v", name, err)
	}
	md, ok := d.(protoreflect.MessageDescriptor)
	if !ok {
		t.Fatalf("%s is not a message", name)
	}
	return md
}

func contractFiles(t *testing.T) []protoreflect.FileDescriptor {
	t.Helper()

	// Reference the package so its descriptors are registered.
	_ = pluginv1.File_nginxui_plugin_v1_options_proto

	var files []protoreflect.FileDescriptor
	protoregistry.GlobalFiles.RangeFilesByPackage(contractPackage, func(fd protoreflect.FileDescriptor) bool {
		files = append(files, fd)
		return true
	})
	if len(files) == 0 {
		t.Fatalf("no file registered for %s", contractPackage)
	}
	return files
}

func contractMessages(t *testing.T) []protoreflect.MessageDescriptor {
	t.Helper()

	var messages []protoreflect.MessageDescriptor
	for _, file := range contractFiles(t) {
		list := file.Messages()
		for i := range list.Len() {
			messages = append(messages, list.Get(i))
		}
	}
	return messages
}

// jsonFields returns the exported fields of a struct keyed by JSON name.
func jsonFields(t reflect.Type) map[string]reflect.StructField {
	fields := map[string]reflect.StructField{}
	for i := range t.NumField() {
		sf := t.Field(i)
		if !sf.IsExported() {
			continue
		}
		name, _, _ := strings.Cut(sf.Tag.Get("json"), ",")
		if name == "-" {
			continue
		}
		if name == "" {
			name = sf.Name
		}
		fields[name] = sf
	}
	return fields
}

// compatible reports whether a Go field and a proto field share a JSON shape.
func compatible(t reflect.Type, fd protoreflect.FieldDescriptor, aligned map[reflect.Type][]protoreflect.FullName) bool {
	t = deref(t)
	switch {
	case fd.IsMap():
		return t.Kind() == reflect.Map && t.Key().Kind() == reflect.String && compatibleSingular(t.Elem(), fd.MapValue(), aligned)
	case fd.IsList():
		return t.Kind() == reflect.Slice && compatibleSingular(t.Elem(), fd, aligned)
	default:
		return compatibleSingular(t, fd, aligned)
	}
}

func compatibleSingular(t reflect.Type, fd protoreflect.FieldDescriptor, aligned map[reflect.Type][]protoreflect.FullName) bool {
	t = deref(t)
	switch fd.Kind() {
	case protoreflect.StringKind, protoreflect.BytesKind:
		// bytes maps to a base64 JSON string.
		return t.Kind() == reflect.String
	case protoreflect.BoolKind:
		return t.Kind() == reflect.Bool
	case protoreflect.DoubleKind, protoreflect.FloatKind:
		// A byte size is a double on the wire and a whole number in Go.
		switch t.Kind() {
		case reflect.Float32, reflect.Float64, reflect.Int64, reflect.Uint64:
			return true
		}
		return false
	case protoreflect.Int32Kind, protoreflect.Sint32Kind, protoreflect.Sfixed32Kind,
		protoreflect.Uint32Kind, protoreflect.Fixed32Kind:
		switch t.Kind() {
		case reflect.Int, reflect.Int8, reflect.Int16, reflect.Int32, reflect.Int64,
			reflect.Uint, reflect.Uint8, reflect.Uint16, reflect.Uint32, reflect.Uint64:
			return true
		}
		return false
	case protoreflect.MessageKind:
		switch fd.Message().FullName() {
		case "google.protobuf.Value":
			return t.Kind() == reflect.Interface
		case "google.protobuf.Struct":
			// Error.Data is typed any on the Go side.
			return t.Kind() == reflect.Interface ||
				(t.Kind() == reflect.Map && t.Key().Kind() == reflect.String && t.Elem().Kind() == reflect.Interface)
		case "google.protobuf.ListValue":
			return t.Kind() == reflect.Slice
		}
		return t.Kind() == reflect.Struct && slices.Contains(aligned[t], fd.Message().FullName())
	}
	return false
}

func deref(t reflect.Type) reflect.Type {
	for t.Kind() == reflect.Pointer {
		t = t.Elem()
	}
	return t
}

func describe(fd protoreflect.FieldDescriptor) string {
	kind := fd.Kind().String()
	if fd.Kind() == protoreflect.MessageKind {
		kind = string(fd.Message().FullName())
	}
	switch {
	case fd.IsMap():
		return "map<string, " + describe(fd.MapValue()) + ">"
	case fd.IsList():
		return "repeated " + kind
	}
	return kind
}

// missing returns the names of a that are not in b, sorted.
func missing(a, b []string) []string {
	var out []string
	for _, name := range a {
		if !slices.Contains(b, name) {
			out = append(out, name)
		}
	}
	sort.Strings(out)
	return out
}
