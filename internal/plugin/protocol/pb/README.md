# pb

Go bindings of the plugin contract, generated from the proto files in
[nginx-ui-plugin-spec](https://github.com/0xJacky/nginx-ui-plugin-spec)
(`proto/nginxui/plugin/v1`). The proto is the single source of truth for the
wire protocol; the hand-written types in `internal/plugin/protocol` mirror it
and `internal/plugin/protocol/alignment_test.go` fails when they drift.

Every `*.pb.go` file here is a verbatim copy of
`gen/go/nginxui/plugin/v1` in the spec repository. Do not edit them. The Go
package name is `pluginv1`, as generated.

## Updating

1. Change the proto in the spec repository and run `make generate` there.
2. Run `./regen.sh` here. It expects the spec checkout next to this
   repository (`../nginx-ui-plugin-spec`); set `SPEC_DIR` to use another path.
3. Update the hand-written types in `internal/plugin/protocol` and the
   alignment table in `alignment_test.go`, then run
   `GOWORK=off go test ./internal/plugin/protocol/...`.

`./regen.sh --check` only reports files that differ from the spec repository.

## Linking

The package registers its descriptors in the global protobuf registry. Do not
link it into one binary together with another copy of the same package (the
plugin SDK's `pb` or the spec repository's `gen/go`): the Go protobuf runtime
refuses duplicate registrations of `nginxui.plugin.v1` at start-up.
