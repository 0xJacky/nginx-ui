---
outline: [2, 3]
---

# Check Rules

`nginx-ui plugin lint` and `nginx-ui plugin conformance` report every problem
with the name of the rule it breaks. This page explains each rule and links to
the part of the guide it comes from.

- <Badge type="info" text="lint" /> rules are checked from the files of a
  plugin directory or package. An <Badge type="danger" text="error" /> stops
  the package from installing; a <Badge type="warning" text="warning" />
  points at a problem the person installing the plugin will notice.
- <Badge type="tip" text="conformance" /> rules are checked by running the
  plugin. A case passes, fails, or is skipped when it does not apply.

## Manifest

### manifest-json

<Badge type="info" text="lint" /> <Badge type="danger" text="error" />

`plugin.json` is missing or is not a single JSON object. See
[Manifest](./manifest.md).

### manifest-id

<Badge type="info" text="lint" /> <Badge type="danger" text="error" />

`id` is missing, longer than 64 characters or not a dotted
lowercase id such as `io.github.example.mydns`. See
[Naming](./naming.md#plugin-ids).

### manifest-name

<Badge type="info" text="lint" /> <Badge type="danger" text="error" />

`name` is missing or empty.

### manifest-version

<Badge type="info" text="lint" /> <Badge type="danger" text="error" />

`version` is missing or not a semantic version.

### manifest-api-version

<Badge type="info" text="lint" /> <Badge type="danger" text="error" />

`api_version` is missing or is not the protocol generation
Nginx UI implements. See [Versions and Compatibility](./versioning.md).

### manifest-parts

<Badge type="info" text="lint" /> <Badge type="danger" text="error" />

The manifest declares none of `server`, `webapp` and `content`,
so the plugin would install nothing.

### manifest-icon

<Badge type="info" text="lint" /> <Badge type="danger" text="error" />

`icon_path` is not a [safe relative path](./packaging.md#safe-paths).

### manifest-i18n

<Badge type="info" text="lint" /> <Badge type="danger" text="error" />

An `i18n` key is not a language of the Nginx UI interface. See
[Manifest](./manifest.md#translations).

### manifest-requires

<Badge type="info" text="lint" /> <Badge type="danger" text="error" />

An entry of `requires` has an invalid plugin id. See
[Dependencies and Conflicts](./manifest.md#dependencies-and-conflicts).

### manifest-conflicts

<Badge type="info" text="lint" /> <Badge type="danger" text="error" />

An entry of `conflicts` is not a valid plugin id, is the plugin's
own id, is listed twice or is also in `requires`. See
[Dependencies and Conflicts](./manifest.md#dependencies-and-conflicts).

### manifest-permissions

<Badge type="info" text="lint" /> <Badge type="danger" text="error" /> <Badge type="warning" text="warning" />

An unknown permission is an error, a permission listed twice a warning.
See [Permissions and Security](./permissions.md#permissions).

### capability-name

<Badge type="info" text="lint" /> <Badge type="danger" text="error" />

A capability name is not one Nginx UI knows. See
[Manifest](./manifest.md#capabilities).

### capability-duplicate

<Badge type="info" text="lint" /> <Badge type="danger" text="error" />

A capability is listed twice.

### server-entry

<Badge type="info" text="lint" /> <Badge type="danger" text="error" />

`server` has neither `executables` nor `command`. See
[Server Process](./manifest.md#server-process).

### server-command

<Badge type="info" text="lint" /> <Badge type="danger" text="error" />

The first element of `server.command` is empty.

### server-lifecycle

<Badge type="info" text="lint" /> <Badge type="danger" text="error" />

`server.lifecycle` is not `resident` or `on_demand`.

### server-idle-timeout

<Badge type="info" text="lint" /> <Badge type="danger" text="error" />

`server.idle_timeout_seconds` is negative.

### server-paths

<Badge type="info" text="lint" /> <Badge type="danger" text="error" /> <Badge type="warning" text="warning" />

Error when an executable path, or the first element of `command` when
it contains a separator, is not a safe relative path. Warning when the
program `command` names is not found on `PATH` of the machine running the
linter.

### server-resources

<Badge type="info" text="lint" /> <Badge type="danger" text="error" />

A value of `server.resources` is negative. See
[Resources](./manifest.md#resources).

### settings-key

<Badge type="info" text="lint" /> <Badge type="danger" text="error" />

A settings field has no key, or two fields share one. See
[Settings](./manifest.md#settings).

### settings-type

<Badge type="info" text="lint" /> <Badge type="danger" text="error" /> <Badge type="warning" text="warning" />

Error for an unknown field type or a `list` default that is not a list
of strings; warning for a default that does not fit its type.

### settings-options

<Badge type="info" text="lint" /> <Badge type="danger" text="error" />

A `select` field has no options.

### event-permission

<Badge type="info" text="lint" /> <Badge type="warning" text="warning" />

`log.paths_changed` is subscribed without the `log.files`
permission, so it is never delivered. See [Host API](./host-api.md#events).

### reserved-namespace

<Badge type="info" text="lint" /> <Badge type="warning" text="warning" />

The id uses `com.nginxui.*`, which is reserved for the Nginx UI
project. See [Naming](./naming.md#plugin-ids).

### network-permission

<Badge type="info" text="lint" /> <Badge type="warning" text="warning" />

A capability that talks to a vendor or target is declared
without the `network` permission. See
[Permissions and Security](./permissions.md#permissions).

### unused-permission

<Badge type="info" text="lint" /> <Badge type="warning" text="warning" />

A permission is requested that only a capability the plugin
does not declare would use, such as `mcp` without the `mcp` capability.

## Package

### package-file-name

<Badge type="info" text="lint" /> <Badge type="warning" text="warning" />

The package file name names another id or version than its
manifest. See [Naming](./naming.md#package-file-names).

### package-layout

<Badge type="info" text="lint" /> <Badge type="danger" text="error" />

`plugin.json` is neither at the root of the archive nor one
directory down, or the archive cannot be read. See
[Packaging](./packaging.md#layout).

### package-paths

<Badge type="info" text="lint" /> <Badge type="danger" text="error" />

An archive entry is not a
[safe relative path](./packaging.md#safe-paths).

### package-links

<Badge type="info" text="lint" /> <Badge type="danger" text="error" />

The archive contains a symbolic or hard link.

### package-escape

<Badge type="info" text="lint" /> <Badge type="danger" text="error" />

An entry would be extracted outside the plugin directory.

### package-entries

<Badge type="info" text="lint" /> <Badge type="danger" text="error" />

The package has more than 10,000 entries. See
[Limits](./packaging.md#limits).

### package-size

<Badge type="info" text="lint" /> <Badge type="danger" text="error" />

The files of the package exceed 256 MiB uncompressed.

### package-docs

<Badge type="info" text="lint" /> <Badge type="danger" text="error" /> <Badge type="warning" text="warning" />

Error when `README.md` is missing; warning when `LICENSE` is missing, or
when the readme lacks a section it should have.
See [Packaging](./packaging.md#layout).

### package-executables

<Badge type="info" text="lint" /> <Badge type="danger" text="error" /> <Badge type="warning" text="warning" />

Error when a file `server.executables` or `server.command` declares is
missing or not a regular file; warning when it lacks the executable bit. See
[Executables](./packaging.md#executables).

### package-platform

<Badge type="info" text="lint" /> <Badge type="danger" text="error" />

A per-platform package does not declare exactly its own platform
in `server.executables`. See
[Per-Platform Packages](./packaging.md#per-platform-packages).

## Signature

### signature-sums

<Badge type="info" text="lint" /> <Badge type="danger" text="error" />

`plugin.sums` does not follow the required format. See
[Signing by Hand](./signing.md#signing-by-hand).

### signature-files

<Badge type="info" text="lint" /> <Badge type="warning" text="warning" />

Only one of `plugin.sums` and `plugin.sums.minisig` is present,
so the package counts as unsigned.

### signature-mismatch

<Badge type="info" text="lint" /> <Badge type="danger" text="error" />

`plugin.sums` does not match the files: a listed file is missing
or has another SHA-256, or a file is not listed. The package changed after
signing and will never install. See
[How Nginx UI Checks a Package](./signing.md#how-nginx-ui-checks-a-package).

### signature-signer

<Badge type="info" text="lint" /> <Badge type="warning" text="warning" />

The signature does not verify with a key the linter knows: the
official plugin key of the project, or a partner key with a valid certificate
in the package. This is expected for a community plugin, whose key only a
catalog or an operator names. See [Trust Levels](./signing.md#trust-levels).

### partner-files

<Badge type="info" text="lint" /> <Badge type="warning" text="warning" />

Only one of `plugin.partner` and `plugin.partner.minisig` is
present, so the package carries no partner certificate. See
[Partner Plugins](./signing.md#partner-plugins).

### partner-comment

<Badge type="info" text="lint" /> <Badge type="warning" text="warning" />

The partner certificate does not verify: `plugin.partner` is
not a public key, the official plugin key did not sign it, or the trusted
comment has the wrong format.

### partner-certificate

<Badge type="info" text="lint" /> <Badge type="warning" text="warning" />

The partner certificate has expired, or `plugin.sums.minisig`
is not signed by the key the certificate names, so the certificate gives the
package nothing.

## Protocol and Lifecycle

### protocol-stderr

<Badge type="tip" text="conformance" />

The plugin writes its logs to standard error. See
[Protocol](./protocol.md#framing).

### protocol-notification

<Badge type="tip" text="conformance" />

A notification of an unknown method gets no answer and the
connection stays usable. See [Messages](./protocol.md#messages).

### protocol-concurrency

<Badge type="tip" text="conformance" />

20 concurrent `plugin.ping` calls are all answered.

### protocol-errors

<Badge type="tip" text="conformance" />

A method the plugin does not have answers `-32601`, and
malformed parameters answer `-32602` or `-32000` instead of hanging. See
[Error Codes](./protocol.md#error-codes).

### protocol-grpc

<Badge type="tip" text="conformance" />

The gRPC endpoint the plugin reports accepts a connection and
answers `plugin.ping`. See [gRPC Transport](./protocol.md#grpc-transport).

### protocol-transports

<Badge type="tip" text="conformance" />

The same call gives the same result or error over standard input
and output and over gRPC. Runs when both transports were tested.

### lifecycle-handshake

<Badge type="tip" text="conformance" />

The handshake completes within its time limit. See
[Lifecycle](./lifecycle.md#handshake).

### lifecycle-api-version

<Badge type="tip" text="conformance" />

The handshake reports the protocol generation Nginx UI
implements.

### lifecycle-capabilities

<Badge type="tip" text="conformance" />

The handshake reports exactly the capabilities of the manifest.

### lifecycle-ping

<Badge type="tip" text="conformance" />

`plugin.ping` is answered. See
[Health Checks](./lifecycle.md#health-checks).

### lifecycle-shutdown

<Badge type="tip" text="conformance" />

`plugin.shutdown` and `plugin.exit` stop the process in time. See
[Stopping](./lifecycle.md#stopping).

## Web Interface

### webapp-paths

<Badge type="info" text="lint" /> <Badge type="danger" text="error" />

`webapp.bundle_path` or `webapp.style_path` is not a safe
relative path. See [Browser Bundle](./webapp.md).

### webapp-pages

<Badge type="info" text="lint" /> <Badge type="danger" text="error" />

A page has no `path`, or its `file` is not a safe relative path.
See [Static Pages](./pages.md).

### webapp-chunks

<Badge type="info" text="lint" /> <Badge type="danger" text="error" />

`webapp.chunks` is present without `bundle_path`, or a chunk has
an invalid name or path, points at the bundle itself, shares a path with
another chunk or names a missing file. See [Chunks](./webapp.md#chunks).

### webapp-bundle

<Badge type="info" text="lint" /> <Badge type="tip" text="conformance" />

The bundle file exists and is a script Nginx UI can
load. See [Building](./webapp.md#building).

### webapp-register

<Badge type="tip" text="conformance" />

The bundle registers itself with the manifest id. See
[Registering the Plugin](./webapp.md#registering-the-plugin).

### webapp-chunk-files

<Badge type="info" text="lint" /> <Badge type="tip" text="conformance" />

Error when a chunk file is empty or missing; warning
when it lies outside the directory Nginx UI serves the bundle from.

## DNS-01

### dns01-block

<Badge type="info" text="lint" /> <Badge type="danger" text="error" />

The `dns01` capability is declared without providers. See
[DNS-01](./capabilities/dns01.md#declaring-providers).

### dns01-code

<Badge type="info" text="lint" /> <Badge type="danger" text="error" />

A provider code does not match `^[a-z0-9-]{2,32}$` or is declared
twice.

### dns01-name

<Badge type="info" text="lint" /> <Badge type="danger" text="error" />

A provider has no name.

### dns01-form

<Badge type="info" text="lint" /> <Badge type="danger" text="error" />

A provider has no `form`, or the form breaks a rule of the
[Credential Form](./capabilities/dns01.md#credential-form): a field without a
key or label, a duplicate key, an unknown `group` or `unit`, a single sign in
method, duplicate method names, a method listing a field that is not a
credential, a misused `values` key, two identical methods or more than one
recommended method.

### dns01-present

<Badge type="tip" text="conformance" />

`dns01.present` answers for the first provider.

### dns01-validate

<Badge type="tip" text="conformance" />

`dns01.validate` with an empty configuration answers as
expected.

### dns01-options

<Badge type="tip" text="conformance" />

`dns01.options` answers, or answers `-32002`.

### dns01-check

<Badge type="tip" text="conformance" />

`dns01.check` answers, or answers `-32002`.

## HTTP Endpoints

### http-block

<Badge type="info" text="lint" /> <Badge type="danger" text="error" />

The `http` capability is declared without an `http` block, or
`listen` is not `unix` or `rpc`. See [HTTP Endpoints](./http.md).

## Configuration Forms

### configuration-fields

<Badge type="info" text="lint" /> <Badge type="danger" text="error" />

A field of a `configuration` form has no key, a duplicate key,
an unknown type or no `display_name`. See
[Configuration Form](./capabilities/notify.md#configuration-form).

## Notification Channels

### notify-block

<Badge type="info" text="lint" /> <Badge type="danger" text="error" />

The `notify` capability is declared without channels. See
[Notification Channels](./capabilities/notify.md).

### notify-code

<Badge type="info" text="lint" /> <Badge type="danger" text="error" />

A channel code does not match `^[a-z0-9-]{2,32}$` or is declared
twice.

### notify-name

<Badge type="info" text="lint" /> <Badge type="danger" text="error" />

A channel has no name.

### notify-validate

<Badge type="tip" text="conformance" />

`notify.validate` with an empty configuration answers `-32003`
naming a required field, or `{}` when the channel has none. `notify.send` is
never called, since it would reach the service.

## Health Checks

### probe-block

<Badge type="info" text="lint" /> <Badge type="danger" text="error" />

The `probe` capability is declared without kinds. See
[Health Checks](./capabilities/probe.md).

### probe-code

<Badge type="info" text="lint" /> <Badge type="danger" text="error" />

A kind code does not match `^[a-z0-9-]{2,32}$` or is declared
twice.

### probe-kind

<Badge type="info" text="lint" /> <Badge type="danger" text="error" />

A kind has no name, or its configuration form is invalid.

### probe-check

<Badge type="tip" text="conformance" />

`probe.check` of the first kind against an unreachable target
answers in time.

### probe-result

<Badge type="tip" text="conformance" />

The answer has a known `status` and a non-negative
`latency_ms`, or is `-32003` naming a field.

## MCP Tools

### mcp-block

<Badge type="info" text="lint" /> <Badge type="danger" text="error" />

The `mcp` capability is declared without tools or without the
`mcp` permission. See [MCP Tools](./capabilities/mcp.md).

### mcp-tool

<Badge type="info" text="lint" /> <Badge type="danger" text="error" />

A tool name does not match `^[a-z0-9][a-z0-9_-]{0,47}$`, is
declared twice, or the tool has no description.

### mcp-input-schema

<Badge type="info" text="lint" /> <Badge type="danger" text="error" />

An `input_schema` whose `type` is not `object`.

### mcp-unknown-tool

<Badge type="tip" text="conformance" />

Calling a tool the manifest does not declare answers `-32602`.
No declared tool is called, since it may change state.

## Storage

### storage-block

<Badge type="info" text="lint" /> <Badge type="danger" text="error" />

The `storage` capability is declared without backends. See
[Storage](./capabilities/storage.md).

### storage-code

<Badge type="info" text="lint" /> <Badge type="danger" text="error" />

A backend code does not match `^[a-z0-9-]{2,32}$` or is declared
twice.

### storage-backend

<Badge type="info" text="lint" /> <Badge type="danger" text="error" />

A backend has no name, or its configuration form is invalid.

### storage-list

<Badge type="tip" text="conformance" />

`storage.list` with an empty configuration answers in time,
with `-32003` naming a required field or with a list. Nothing is stored,
fetched or deleted.

### storage-validate

<Badge type="tip" text="conformance" />

`storage.validate` with an empty configuration answers `-32003`
naming a required field, or `{}`.

## Certificate Deployment

### deploy-block

<Badge type="info" text="lint" /> <Badge type="danger" text="error" />

The `cert.deploy` capability is declared without targets or
without the `cert.deploy` permission. See
[Certificate Deployment](./capabilities/cert-deploy.md).

### deploy-code

<Badge type="info" text="lint" /> <Badge type="danger" text="error" />

A target code does not match `^[a-z0-9-]{2,32}$` or is declared
twice.

### deploy-target

<Badge type="info" text="lint" /> <Badge type="danger" text="error" />

A target has no name, or its configuration form is invalid.

### deploy-dry-run

<Badge type="tip" text="conformance" />

A dry run push of a throwaway certificate answers in time. A
real push is never made.

### deploy-validate

<Badge type="tip" text="conformance" />

`deploy.validate` with an empty configuration answers `-32003`
naming a required field, or `{}`.

## Blocklists

### blocklist-block

<Badge type="info" text="lint" /> <Badge type="danger" text="error" />

The `security.blocklist` capability is declared without sources
or without the `network` permission. See [Blocklists](./capabilities/blocklist.md).

### blocklist-code

<Badge type="info" text="lint" /> <Badge type="danger" text="error" />

A source code does not match `^[a-z0-9-]{2,32}$` or is declared
twice.

### blocklist-source

<Badge type="info" text="lint" /> <Badge type="danger" text="error" />

A source has no name, an invalid configuration form, or a
`refresh_seconds` between 1 and 59 or below 0.

### blocklist-fetch

<Badge type="tip" text="conformance" />

`blocklist.fetch` with an empty configuration answers in time
with entries whose addresses parse.

### blocklist-errors

<Badge type="tip" text="conformance" />

A source that needs configuration answers `-32003` naming the
field instead of an empty list.

## Upstream Discovery

### discovery-block

<Badge type="info" text="lint" /> <Badge type="danger" text="error" />

The `upstream.discovery` capability is declared without
providers or without the `network` permission. See
[Upstream Discovery](./capabilities/discovery.md).

### discovery-code

<Badge type="info" text="lint" /> <Badge type="danger" text="error" />

A provider code does not match `^[a-z0-9-]{2,32}$` or is
declared twice.

### discovery-provider

<Badge type="info" text="lint" /> <Badge type="danger" text="error" />

A provider has no name, or its configuration form is invalid.

### discovery-resolve

<Badge type="tip" text="conformance" />

`discovery.resolve` with an empty configuration answers in time
with targets whose ports are between 1 and 65535.

### discovery-errors

<Badge type="tip" text="conformance" />

A provider that needs configuration, or an unknown service,
answers `-32003` naming the field.

## Access Log Streaming

### log-sink-permission

<Badge type="info" text="lint" /> <Badge type="danger" text="error" />

The `log.sink` capability is declared without the `log.read`
permission. See [Access Log Streaming](./capabilities/log-sink.md).

### log-sink-block

<Badge type="info" text="lint" /> <Badge type="warning" text="warning" />

A `log_sink` block without the `log.sink` capability, which
has no effect.

### log-sink-batch

<Badge type="info" text="lint" /> <Badge type="danger" text="error" />

`batch_size` is outside 0 to 4096, or `flush_interval_ms` is
between 1 and 49 or negative.

### log-sink-formats

<Badge type="info" text="lint" /> <Badge type="danger" text="error" />

A format is not `combined` or `raw`, or is listed twice.

### log-sink-transport

<Badge type="tip" text="conformance" />

`log.push` sent over standard input and output answers `-32601`,
and the plugin serves gRPC.

### log-sink-push

<Badge type="tip" text="conformance" />

A stream of three entries is answered in time with `accepted`
3. Skipped when only standard input and output are tested.

## Templates and Translations

### content-paths

<Badge type="info" text="lint" /> <Badge type="danger" text="error" />

`content.templates` or `content.locales` is not a safe relative
path. See [Templates and Translations](./capabilities/content.md).

### content-without-server

<Badge type="info" text="lint" /> <Badge type="danger" text="error" />

A manifest without `server` declares `capabilities`, `cron` or
`events`, which need a process.

### content-templates

<Badge type="info" text="lint" /> <Badge type="danger" text="error" /> <Badge type="warning" text="warning" />

Error when the templates directory is missing, is not a directory or
holds no template in `conf/` or `block/`; warning for an entry that is not a
template. See [Templates](./capabilities/content.md#templates).

### content-template

<Badge type="info" text="lint" /> <Badge type="danger" text="error" /> <Badge type="warning" text="warning" />

Error when a template does not parse or render with its default
values; warning when it has no `name`.

### content-locales

<Badge type="info" text="lint" /> <Badge type="danger" text="error" /> <Badge type="warning" text="warning" />

Error when the locales directory is missing, holds no `.po` file or
holds one named after a language Nginx UI does not have; warning for other
entries. See [Translations](./capabilities/content.md#translations).

### content-locale

<Badge type="info" text="lint" /> <Badge type="danger" text="error" />

A translation file does not parse.
