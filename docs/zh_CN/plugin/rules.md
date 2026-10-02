---
outline: [2, 3]
---

# 检查规则

`nginx-ui plugin lint` 和 `nginx-ui plugin conformance` 报告的每个问题都附带它违反的规则名称。本页解释每一条规则，并链接到指南中的相关内容。

- <Badge type="info" text="lint" /> 规则根据插件目录或插件包中的文件检查。<Badge type="danger" text="error" /> 会阻止插件包安装；<Badge type="warning" text="warning" /> 指出安装插件的人会注意到的问题。
- <Badge type="tip" text="conformance" /> 规则通过运行插件来检查。每个用例的结果为通过、失败，或在不适用时跳过。

## 清单 {#manifest}

### manifest-json

<Badge type="info" text="lint" /> <Badge type="danger" text="error" />

`plugin.json` 缺失，或不是单个 JSON 对象。参见[清单文件](./manifest.md)。

### manifest-id

<Badge type="info" text="lint" /> <Badge type="danger" text="error" />

`id` 缺失、超过 64 个字符，或不是 `io.github.example.mydns` 这样以点分隔的小写 ID。参见[命名规则](./naming.md#plugin-ids)。

### manifest-name

<Badge type="info" text="lint" /> <Badge type="danger" text="error" />

`name` 缺失或为空。

### manifest-version

<Badge type="info" text="lint" /> <Badge type="danger" text="error" />

`version` 缺失，或不是语义化版本。

### manifest-api-version

<Badge type="info" text="lint" /> <Badge type="danger" text="error" />

`api_version` 缺失，或不是 Nginx UI 实现的协议代次。参见[版本与兼容性](./versioning.md)。

### manifest-parts

<Badge type="info" text="lint" /> <Badge type="danger" text="error" />

清单没有声明 `server`、`webapp` 和 `content` 中的任何一个，插件什么也不会安装。

### manifest-icon

<Badge type="info" text="lint" /> <Badge type="danger" text="error" />

`icon_path` 不是[安全的相对路径](./packaging.md#safe-paths)。

### manifest-i18n

<Badge type="info" text="lint" /> <Badge type="danger" text="error" />

`i18n` 的某个键不是 Nginx UI 界面的语言。参见[清单文件](./manifest.md#translations)。

### manifest-requires

<Badge type="info" text="lint" /> <Badge type="danger" text="error" />

`requires` 中某一项的插件 ID 无效。参见[依赖与冲突](./manifest.md#dependencies-and-conflicts)。

### manifest-conflicts

<Badge type="info" text="lint" /> <Badge type="danger" text="error" />

`conflicts` 中某一项不是有效的插件 ID、是插件自己的 ID、重复出现，或同时出现在 `requires` 中。参见[依赖与冲突](./manifest.md#dependencies-and-conflicts)。

### manifest-permissions

<Badge type="info" text="lint" /> <Badge type="danger" text="error" /> <Badge type="warning" text="warning" />

未知权限为错误，重复列出的权限为警告。参见[权限与安全](./permissions.md#permissions)。

### capability-name

<Badge type="info" text="lint" /> <Badge type="danger" text="error" />

某个能力名称不是 Nginx UI 认识的能力。参见[清单文件](./manifest.md#capabilities)。

### capability-duplicate

<Badge type="info" text="lint" /> <Badge type="danger" text="error" />

某个能力被列出了两次。

### server-entry

<Badge type="info" text="lint" /> <Badge type="danger" text="error" />

`server` 既没有 `executables` 也没有 `command`。参见[服务端进程](./manifest.md#server-process)。

### server-command

<Badge type="info" text="lint" /> <Badge type="danger" text="error" />

`server.command` 的第一个元素为空。

### server-lifecycle

<Badge type="info" text="lint" /> <Badge type="danger" text="error" />

`server.lifecycle` 不是 `resident` 或 `on_demand`。

### server-idle-timeout

<Badge type="info" text="lint" /> <Badge type="danger" text="error" />

`server.idle_timeout_seconds` 为负数。

### server-paths

<Badge type="info" text="lint" /> <Badge type="danger" text="error" /> <Badge type="warning" text="warning" />

可执行文件路径，或包含分隔符的 `command` 第一个元素不是安全的相对路径时为错误；在运行检查工具的机器上，`command` 指定的程序在 `PATH` 中找不到时为警告。

### server-resources

<Badge type="info" text="lint" /> <Badge type="danger" text="error" />

`server.resources` 的某个值为负数。参见[资源](./manifest.md#resources)。

### settings-key

<Badge type="info" text="lint" /> <Badge type="danger" text="error" />

某个设置字段没有键，或两个字段使用了同一个键。参见[设置](./manifest.md#settings)。

### settings-type

<Badge type="info" text="lint" /> <Badge type="danger" text="error" /> <Badge type="warning" text="warning" />

字段类型未知，或 `list` 的默认值不是字符串数组时为错误；默认值与类型不符时为警告。

### settings-options

<Badge type="info" text="lint" /> <Badge type="danger" text="error" />

`select` 字段没有选项。

### event-permission

<Badge type="info" text="lint" /> <Badge type="warning" text="warning" />

订阅了 `log.paths_changed` 却没有 `log.files` 权限，因此该事件永远不会送达。参见[宿主 API](./host-api.md#events)。

### reserved-namespace

<Badge type="info" text="lint" /> <Badge type="warning" text="warning" />

ID 使用了保留给 Nginx UI 项目的 `com.nginxui.*`。参见[命名规则](./naming.md#plugin-ids)。

### network-permission

<Badge type="info" text="lint" /> <Badge type="warning" text="warning" />

与服务商或目标通信的能力没有声明 `network` 权限。参见[权限与安全](./permissions.md#permissions)。

### unused-permission

<Badge type="info" text="lint" /> <Badge type="warning" text="warning" />

请求了只有插件未声明的能力才会用到的权限，例如没有 `mcp` 能力却请求 `mcp`。

## 插件包 {#package}

### package-file-name

<Badge type="info" text="lint" /> <Badge type="warning" text="warning" />

插件包文件名中的 ID 或版本与清单不一致。参见[命名规则](./naming.md#package-file-names)。

### package-layout

<Badge type="info" text="lint" /> <Badge type="danger" text="error" />

`plugin.json` 既不在归档根目录，也不在一层目录之下，或者归档无法读取。参见[打包](./packaging.md#layout)。

### package-paths

<Badge type="info" text="lint" /> <Badge type="danger" text="error" />

某个归档条目不是[安全的相对路径](./packaging.md#safe-paths)。

### package-links

<Badge type="info" text="lint" /> <Badge type="danger" text="error" />

归档中包含符号链接或硬链接。

### package-escape

<Badge type="info" text="lint" /> <Badge type="danger" text="error" />

某个条目会被解压到插件目录之外。

### package-entries

<Badge type="info" text="lint" /> <Badge type="danger" text="error" />

插件包的条目超过 10,000 个。参见[限制](./packaging.md#limits)。

### package-size

<Badge type="info" text="lint" /> <Badge type="danger" text="error" />

插件包中的文件解压后超过 256 MiB。

### package-docs

<Badge type="info" text="lint" /> <Badge type="danger" text="error" /> <Badge type="warning" text="warning" />

缺少 `README.md` 时为错误；缺少 `LICENSE`，或者 README 缺少应有的章节时为警告。参见[打包](./packaging.md#layout)。

### package-executables

<Badge type="info" text="lint" /> <Badge type="danger" text="error" /> <Badge type="warning" text="warning" />

`server.executables` 或 `server.command` 声明的文件缺失或不是普通文件时为错误；缺少可执行权限时为警告。参见[可执行文件](./packaging.md#executables)。

### package-platform

<Badge type="info" text="lint" /> <Badge type="danger" text="error" />

单一平台的插件包没有在 `server.executables` 中恰好只声明自己的平台。参见[单一平台的插件包](./packaging.md#per-platform-packages)。

## 签名 {#signature}

### signature-sums

<Badge type="info" text="lint" /> <Badge type="danger" text="error" />

`plugin.sums` 不符合规定的格式。参见[手动签名](./signing.md#signing-by-hand)。

### signature-files

<Badge type="info" text="lint" /> <Badge type="warning" text="warning" />

`plugin.sums` 和 `plugin.sums.minisig` 只有一个，因此插件包被视为未签名。

### signature-mismatch

<Badge type="info" text="lint" /> <Badge type="danger" text="error" />

`plugin.sums` 与文件不符：列出的文件缺失或 SHA-256 不同，或者有文件没有列出。插件包在签名后被修改过，永远无法安装。参见 [Nginx UI 如何检查插件包](./signing.md#how-nginx-ui-checks-a-package)。

### signature-signer

<Badge type="info" text="lint" /> <Badge type="warning" text="warning" />

签名无法用检查工具已知的密钥验证：项目的官方插件签名密钥，或插件包中带有有效证书的合作伙伴密钥。对于社区插件这是预期的，因为它的密钥只由插件目录或运维人员指定。参见[信任等级](./signing.md#trust-levels)。

### partner-files

<Badge type="info" text="lint" /> <Badge type="warning" text="warning" />

`plugin.partner` 和 `plugin.partner.minisig` 只有一个，因此插件包没有合作伙伴证书。参见[合作伙伴插件](./signing.md#partner-plugins)。

### partner-comment

<Badge type="info" text="lint" /> <Badge type="warning" text="warning" />

合作伙伴证书无法验证：`plugin.partner` 不是公钥，没有官方插件签名密钥为它签名，或者可信注释的格式错误。

### partner-certificate

<Badge type="info" text="lint" /> <Badge type="warning" text="warning" />

合作伙伴证书已过期，或者 `plugin.sums.minisig` 不是由证书指定的密钥签名的，因此证书对插件包不起作用。

## 协议与生命周期 {#protocol-and-lifecycle}

### protocol-stderr

<Badge type="tip" text="conformance" />

插件把日志写到标准错误。参见[通信协议](./protocol.md#framing)。

### protocol-notification

<Badge type="tip" text="conformance" />

未知方法的通知不会得到回复，且连接仍然可用。参见[消息](./protocol.md#messages)。

### protocol-concurrency

<Badge type="tip" text="conformance" />

20 个并发的 `plugin.ping` 调用全部得到回复。

### protocol-errors

<Badge type="tip" text="conformance" />

插件没有的方法回复 `-32601`，格式错误的参数回复 `-32602` 或 `-32000`，而不是挂起。参见[错误码](./protocol.md#error-codes)。

### protocol-grpc

<Badge type="tip" text="conformance" />

插件报告的 gRPC 端点接受连接并回复 `plugin.ping`。参见 [gRPC 传输](./protocol.md#grpc-transport)。

### protocol-transports

<Badge type="tip" text="conformance" />

同一个调用通过标准输入输出和通过 gRPC 得到相同的结果或错误。两种传输方式都测试过时才运行。

### lifecycle-handshake

<Badge type="tip" text="conformance" />

握手在时间限制内完成。参见[生命周期](./lifecycle.md#handshake)。

### lifecycle-api-version

<Badge type="tip" text="conformance" />

握手报告的协议代次与 Nginx UI 实现的一致。

### lifecycle-capabilities

<Badge type="tip" text="conformance" />

握手报告的能力与清单完全一致。

### lifecycle-ping

<Badge type="tip" text="conformance" />

`plugin.ping` 得到回复。参见[健康检查](./lifecycle.md#health-checks)。

### lifecycle-shutdown

<Badge type="tip" text="conformance" />

`plugin.shutdown` 和 `plugin.exit` 能及时停止进程。参见[停止](./lifecycle.md#stopping)。

## 网页界面 {#web-interface}

### webapp-paths

<Badge type="info" text="lint" /> <Badge type="danger" text="error" />

`webapp.bundle_path` 或 `webapp.style_path` 不是安全的相对路径。参见[浏览器包](./webapp.md)。

### webapp-pages

<Badge type="info" text="lint" /> <Badge type="danger" text="error" />

某个页面没有 `path`，或其 `file` 不是安全的相对路径。参见[静态页面](./pages.md)。

### webapp-chunks

<Badge type="info" text="lint" /> <Badge type="danger" text="error" />

声明了 `webapp.chunks` 却没有 `bundle_path`，或者某个分块的名称或路径无效、指向浏览器包本身、与其他分块共用路径，或指向缺失的文件。参见[分块](./webapp.md#chunks)。

### webapp-bundle

<Badge type="info" text="lint" /> <Badge type="tip" text="conformance" />

浏览器包文件存在，并且是 Nginx UI 可以加载的脚本。参见[构建](./webapp.md#building)。

### webapp-register

<Badge type="tip" text="conformance" />

浏览器包用清单 ID 注册自己。参见[注册插件](./webapp.md#registering-the-plugin)。

### webapp-chunk-files

<Badge type="info" text="lint" /> <Badge type="tip" text="conformance" />

分块文件为空或缺失时为错误；位于 Nginx UI 提供浏览器包的目录之外时为警告。

## DNS-01 {#dns-01}

### dns01-block

<Badge type="info" text="lint" /> <Badge type="danger" text="error" />

声明了 `dns01` 能力却没有服务商。参见 [DNS-01](./capabilities/dns01.md#declaring-providers)。

### dns01-code

<Badge type="info" text="lint" /> <Badge type="danger" text="error" />

服务商代码不匹配 `^[a-z0-9-]{2,32}$`，或被声明了两次。

### dns01-name

<Badge type="info" text="lint" /> <Badge type="danger" text="error" />

服务商没有名称。

### dns01-form

<Badge type="info" text="lint" /> <Badge type="danger" text="error" />

服务商没有 `form`，或者表单违反了[凭据表单](./capabilities/dns01.md#credential-form)的规则：字段没有键或标签、键重复、`group` 或 `unit` 未知、只有一种登录方式、方式名称重复、方式列出了非凭据字段、`values` 的键使用不当、两种方式完全相同，或者有多个推荐方式。

### dns01-present

<Badge type="tip" text="conformance" />

`dns01.present` 对第一个服务商有回复。

### dns01-validate

<Badge type="tip" text="conformance" />

`dns01.validate` 对空配置的回复符合预期。

### dns01-options

<Badge type="tip" text="conformance" />

`dns01.options` 有回复，或回复 `-32002`。

### dns01-check

<Badge type="tip" text="conformance" />

`dns01.check` 有回复，或回复 `-32002`。

## HTTP 接口 {#http-endpoints}

### http-block

<Badge type="info" text="lint" /> <Badge type="danger" text="error" />

声明了 `http` 能力却没有 `http` 块，或者 `listen` 不是 `unix` 或 `rpc`。参见 [HTTP 接口](./http.md)。

## 配置表单 {#configuration-forms}

### configuration-fields

<Badge type="info" text="lint" /> <Badge type="danger" text="error" />

`configuration` 表单中的某个字段没有键、键重复、类型未知或没有 `display_name`。参见[配置表单](./capabilities/notify.md#configuration-form)。

## 通知渠道 {#notification-channels}

### notify-block

<Badge type="info" text="lint" /> <Badge type="danger" text="error" />

声明了 `notify` 能力却没有渠道。参见[通知渠道](./capabilities/notify.md)。

### notify-code

<Badge type="info" text="lint" /> <Badge type="danger" text="error" />

渠道代码不匹配 `^[a-z0-9-]{2,32}$`，或被声明了两次。

### notify-name

<Badge type="info" text="lint" /> <Badge type="danger" text="error" />

渠道没有名称。

### notify-validate

<Badge type="tip" text="conformance" />

`notify.validate` 对空配置回复指出必填字段的 `-32003`，或者在渠道没有必填字段时回复 `{}`。测试从不调用 `notify.send`，因为它会访问真实服务。

## 健康检查 {#health-checks}

### probe-block

<Badge type="info" text="lint" /> <Badge type="danger" text="error" />

声明了 `probe` 能力却没有类型。参见[健康检查](./capabilities/probe.md)。

### probe-code

<Badge type="info" text="lint" /> <Badge type="danger" text="error" />

类型代码不匹配 `^[a-z0-9-]{2,32}$`，或被声明了两次。

### probe-kind

<Badge type="info" text="lint" /> <Badge type="danger" text="error" />

类型没有名称，或其配置表单无效。

### probe-check

<Badge type="tip" text="conformance" />

对不可达目标执行第一个类型的 `probe.check`，能够及时回复。

### probe-result

<Badge type="tip" text="conformance" />

回复包含已知的 `status` 和非负的 `latency_ms`，或者是指出字段的 `-32003`。

## MCP 工具 {#mcp-tools}

### mcp-block

<Badge type="info" text="lint" /> <Badge type="danger" text="error" />

声明了 `mcp` 能力却没有工具，或没有 `mcp` 权限。参见 [MCP 工具](./capabilities/mcp.md)。

### mcp-tool

<Badge type="info" text="lint" /> <Badge type="danger" text="error" />

工具名称不匹配 `^[a-z0-9][a-z0-9_-]{0,47}$`、被声明了两次，或者工具没有描述。

### mcp-input-schema

<Badge type="info" text="lint" /> <Badge type="danger" text="error" />

`input_schema` 的 `type` 不是 `object`。

### mcp-unknown-tool

<Badge type="tip" text="conformance" />

调用清单未声明的工具会得到 `-32602`。测试不会调用任何已声明的工具，因为它们可能改变状态。

## 存储 {#storage}

### storage-block

<Badge type="info" text="lint" /> <Badge type="danger" text="error" />

声明了 `storage` 能力却没有后端。参见[存储](./capabilities/storage.md)。

### storage-code

<Badge type="info" text="lint" /> <Badge type="danger" text="error" />

后端代码不匹配 `^[a-z0-9-]{2,32}$`，或被声明了两次。

### storage-backend

<Badge type="info" text="lint" /> <Badge type="danger" text="error" />

后端没有名称，或其配置表单无效。

### storage-list

<Badge type="tip" text="conformance" />

使用空配置的 `storage.list` 能及时回复，结果为指出必填字段的 `-32003` 或一个列表。测试不会保存、取回或删除任何内容。

### storage-validate

<Badge type="tip" text="conformance" />

`storage.validate` 对空配置回复指出必填字段的 `-32003`，或回复 `{}`。

## 证书部署 {#certificate-deployment}

### deploy-block

<Badge type="info" text="lint" /> <Badge type="danger" text="error" />

声明了 `cert.deploy` 能力却没有目标，或没有 `cert.deploy` 权限。参见[证书部署](./capabilities/cert-deploy.md)。

### deploy-code

<Badge type="info" text="lint" /> <Badge type="danger" text="error" />

目标代码不匹配 `^[a-z0-9-]{2,32}$`，或被声明了两次。

### deploy-target

<Badge type="info" text="lint" /> <Badge type="danger" text="error" />

目标没有名称，或其配置表单无效。

### deploy-dry-run

<Badge type="tip" text="conformance" />

对一张临时证书的试运行推送能及时回复。测试从不进行真正的推送。

### deploy-validate

<Badge type="tip" text="conformance" />

`deploy.validate` 对空配置回复指出必填字段的 `-32003`，或回复 `{}`。

## 封禁列表 {#blocklists}

### blocklist-block

<Badge type="info" text="lint" /> <Badge type="danger" text="error" />

声明了 `security.blocklist` 能力却没有来源，或没有 `network` 权限。参见[封禁列表](./capabilities/blocklist.md)。

### blocklist-code

<Badge type="info" text="lint" /> <Badge type="danger" text="error" />

来源代码不匹配 `^[a-z0-9-]{2,32}$`，或被声明了两次。

### blocklist-source

<Badge type="info" text="lint" /> <Badge type="danger" text="error" />

来源没有名称、配置表单无效，或者 `refresh_seconds` 介于 1 到 59 之间或为负数。

### blocklist-fetch

<Badge type="tip" text="conformance" />

使用空配置的 `blocklist.fetch` 能及时回复，且条目中的地址都能解析。

### blocklist-errors

<Badge type="tip" text="conformance" />

需要配置的来源回复指出字段的 `-32003`，而不是空列表。

## 上游发现 {#upstream-discovery}

### discovery-block

<Badge type="info" text="lint" /> <Badge type="danger" text="error" />

声明了 `upstream.discovery` 能力却没有提供方，或没有 `network` 权限。参见[上游发现](./capabilities/discovery.md)。

### discovery-code

<Badge type="info" text="lint" /> <Badge type="danger" text="error" />

提供方代码不匹配 `^[a-z0-9-]{2,32}$`，或被声明了两次。

### discovery-provider

<Badge type="info" text="lint" /> <Badge type="danger" text="error" />

提供方没有名称，或其配置表单无效。

### discovery-resolve

<Badge type="tip" text="conformance" />

使用空配置的 `discovery.resolve` 能及时回复，且目标的端口都在 1 到 65535 之间。

### discovery-errors

<Badge type="tip" text="conformance" />

需要配置的提供方或未知的服务回复指出字段的 `-32003`。

## 访问日志推送 {#access-log-streaming}

### log-sink-permission

<Badge type="info" text="lint" /> <Badge type="danger" text="error" />

声明了 `log.sink` 能力却没有 `log.read` 权限。参见[访问日志推送](./capabilities/log-sink.md)。

### log-sink-block

<Badge type="info" text="lint" /> <Badge type="warning" text="warning" />

有 `log_sink` 块却没有 `log.sink` 能力，该块不起作用。

### log-sink-batch

<Badge type="info" text="lint" /> <Badge type="danger" text="error" />

`batch_size` 不在 0 到 4096 之间，或者 `flush_interval_ms` 介于 1 到 49 之间或为负数。

### log-sink-formats

<Badge type="info" text="lint" /> <Badge type="danger" text="error" />

格式不是 `combined` 或 `raw`，或被列出了两次。

### log-sink-transport

<Badge type="tip" text="conformance" />

通过标准输入输出发送的 `log.push` 得到 `-32601`，并且插件提供 gRPC。

### log-sink-push

<Badge type="tip" text="conformance" />

包含三个条目的流能及时得到 `accepted` 为 3 的回复。只测试标准输入输出时跳过。

## 模板与翻译 {#templates-and-translations}

### content-paths

<Badge type="info" text="lint" /> <Badge type="danger" text="error" />

`content.templates` 或 `content.locales` 不是安全的相对路径。参见[模板与翻译](./capabilities/content.md)。

### content-without-server

<Badge type="info" text="lint" /> <Badge type="danger" text="error" />

没有 `server` 的清单声明了 `capabilities`、`cron` 或 `events`，而它们都需要进程。

### content-templates

<Badge type="info" text="lint" /> <Badge type="danger" text="error" /> <Badge type="warning" text="warning" />

模板目录缺失、不是目录，或者 `conf/` 和 `block/` 中都没有模板时为错误；有不是模板的条目时为警告。参见[模板](./capabilities/content.md#templates)。

### content-template

<Badge type="info" text="lint" /> <Badge type="danger" text="error" /> <Badge type="warning" text="warning" />

模板无法解析或无法用默认值渲染时为错误；没有 `name` 时为警告。

### content-locales

<Badge type="info" text="lint" /> <Badge type="danger" text="error" /> <Badge type="warning" text="warning" />

翻译目录缺失、没有 `.po` 文件，或者有以 Nginx UI 不支持的语言命名的文件时为错误；有其他条目时为警告。参见[翻译](./capabilities/content.md#translations)。

### content-locale

<Badge type="info" text="lint" /> <Badge type="danger" text="error" />

某个翻译文件无法解析。
