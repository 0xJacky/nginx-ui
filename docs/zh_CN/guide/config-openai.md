# Open AI

本节用于设置 ChatGPT 配置。请注意，我们不会检查您提供的信息的准确性。如果配置错误，可能会导致 API 请求失败，导致 ChatGPT
助手无法使用。

## Provider

- 类型：`string`
- 默认值：`openai`

此选项用于选择 OpenAI 兼容服务商预设。

- `openai`：使用默认 OpenAI 端点。
- `atlas_cloud`：使用 Atlas Cloud 端点 `https://api.atlascloud.ai/v1`。
- `minimax`：使用 MiniMax 全球 OpenAI 兼容端点 `https://api.minimax.io/v1`。
- `custom`：继续使用自定义 `BaseUrl` 值。

## BaseUrl

- 类型：`string`

此选项用于设置 Open AI API 的基本 URL，如果不需要更改 URL，则将其保留为空。

Atlas Cloud 使用 `https://api.atlascloud.ai/v1`。

MiniMax 的 OpenAI 兼容端点包括全球服务 `https://api.minimax.io/v1` 和中国服务
`https://api.minimaxi.com/v1`。MiniMax 也提供 Anthropic 兼容端点
`https://api.minimax.io/anthropic/v1` 和 `https://api.minimaxi.com/anthropic/v1`；此设置应使用 OpenAI 兼容端点。
建议的 MiniMax 文本模型包括 `MiniMax-M3` 和 `MiniMax-M2.7`。文档见 <https://platform.minimax.io/docs> 和
<https://platform.minimaxi.com/docs>。

## Token

- 类型：`string`

此选项用于设置 Open AI API 的令牌。

## Proxy

- 类型：`string`

此选项用于为 OpenAI 的 API 配置代理。如果您在国家或地区无法访问 OpenAI 的 API，可以使用 HTTP 代理并将此选项设置为相应的
URL。

## Model

- 类型：`string`
- 默认值：`gpt-3.5-turbo`

此选项用于设置对话模型。如果您的帐户有权限访问 `gpt-4` 模型，可以相应地配置此选项。

## APIType

- 类型：`string`
- 默认值：`OPEN_AI`

此选项用于设置 API 的类型。

- `OPEN_AI`: 使用 OpenAI API。
- `AZURE`: 使用 Azure API。

## EnableCodeCompletion

- 类型：`boolean`
- 默认值：`false`
- 版本：`>=2.0.0-rc.6`

此选项用于启用编辑器代码补全功能。

## CodeCompletionModel

- 类型：`string`
- 版本：`>=2.0.0-rc.6`
此选项用于设置代码补全的模型，留空则使用对话模型。
