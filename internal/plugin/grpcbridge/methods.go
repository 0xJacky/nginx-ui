// Package grpcbridge carries JSON-RPC shaped plugin calls over the optional
// gRPC transport (nginx-ui-plugin-spec/spec/03-wire-protocol.md WIRE-11).
//
// Callers keep speaking in JSON-RPC terms: a method name, params and a result
// value. The bridge resolves the method through the proto descriptors of the
// contract, converts the params to the request message, sends the protobuf
// bytes with a raw codec and converts the response or the gRPC status back,
// so a caller sees the same result and the same *protocol.Error on either
// transport.
package grpcbridge

import (
	"fmt"
	"sync"

	pluginv1 "github.com/0xJacky/Nginx-UI/internal/plugin/protocol/pb"
	"google.golang.org/protobuf/proto"
	"google.golang.org/protobuf/reflect/protoreflect"
	"google.golang.org/protobuf/reflect/protoregistry"
)

// Services that never carry capability calls: the lifecycle service drives
// the process over stdio and the host service is served by the host.
var (
	lifecycleService = protoreflect.FullName(pluginv1.Plugin_ServiceDesc.ServiceName)
	hostService      = protoreflect.FullName(pluginv1.Host_ServiceDesc.ServiceName)
)

// Method is one rpc of the plugin contract.
type Method struct {
	// RPCName is the JSON-RPC method, the rpc_name option.
	RPCName string
	// FullMethod is the gRPC path, e.g. /nginxui.plugin.v1.DNS01/Present.
	FullMethod   string
	Service      protoreflect.FullName
	Input        protoreflect.MessageDescriptor
	Output       protoreflect.MessageDescriptor
	Notification bool
	// Streaming marks a client streaming rpc (spec WIRE-12). It has no
	// JSON-RPC form and is opened with Client.OpenStream only.
	Streaming bool
}

// IsCapability reports whether the rpc is a unary host to plugin capability
// request. Only those are routed over gRPC; lifecycle methods, host API calls
// and notifications stay on stdio, and a stream is never routed at all.
func (m *Method) IsCapability() bool {
	return !m.Notification && !m.Streaming && m.Service != lifecycleService && m.Service != hostService
}

type index struct {
	byName map[string]*Method
	byPath map[string]*Method
}

// methods walks the contract descriptors once. Nothing is maintained by
// hand: a new rpc in the proto is routable as soon as the pb copy is updated.
var methods = sync.OnceValue(func() index {
	idx := index{byName: map[string]*Method{}, byPath: map[string]*Method{}}
	pkg := pluginv1.File_nginxui_plugin_v1_options_proto.Package()
	protoregistry.GlobalFiles.RangeFilesByPackage(pkg, func(fd protoreflect.FileDescriptor) bool {
		services := fd.Services()
		for i := range services.Len() {
			sd := services.Get(i)
			rpcs := sd.Methods()
			for j := range rpcs.Len() {
				md := rpcs.Get(j)
				// Only client streams are part of the contract.
				if md.IsStreamingServer() {
					continue
				}
				name, _ := proto.GetExtension(md.Options(), pluginv1.E_RpcName).(string)
				if name == "" {
					continue
				}
				notification, _ := proto.GetExtension(md.Options(), pluginv1.E_Notification).(bool)
				streaming, _ := proto.GetExtension(md.Options(), pluginv1.E_Streaming).(bool)
				m := &Method{
					RPCName:      name,
					FullMethod:   fmt.Sprintf("/%s/%s", sd.FullName(), md.Name()),
					Service:      sd.FullName(),
					Input:        md.Input(),
					Output:       md.Output(),
					Notification: notification,
					Streaming:    streaming || md.IsStreamingClient(),
				}
				idx.byName[m.RPCName] = m
				idx.byPath[m.FullMethod] = m
			}
		}
		return true
	})
	return idx
})

// Lookup resolves a JSON-RPC method name.
func Lookup(rpcName string) (*Method, bool) {
	m, ok := methods().byName[rpcName]
	return m, ok
}

// LookupFullMethod resolves a gRPC path.
func LookupFullMethod(fullMethod string) (*Method, bool) {
	m, ok := methods().byPath[fullMethod]
	return m, ok
}

// IsCapability reports whether rpcName is a unary capability rpc of the
// contract.
func IsCapability(rpcName string) bool {
	m, ok := Lookup(rpcName)
	return ok && m.IsCapability()
}

// IsStreaming reports whether rpcName is a streaming rpc of the contract,
// which travels on gRPC only.
func IsStreaming(rpcName string) bool {
	m, ok := Lookup(rpcName)
	return ok && m.Streaming
}
