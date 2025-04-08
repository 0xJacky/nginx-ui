# Open AI

本节用于设置 ChatGPT 配置。请注意，我们不会检查您提供的信息的准确性。如果配置错误，可能会导致 API 请求失败，导致 ChatGPT
助手无法使用。

## BaseUrl

- 类型：`string`

此选项用于设置 Open AI API 的基本 URL，如果不需要更改 URL，则将其保留为空。

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

此选项用于设置 ChatGPT 模型。如果您的帐户有权限访问 `gpt-4` 模型，可以相应地配置此选项。

## APIType

- 类型：`string`
- 默认值：`OPEN_AI`

此选项用于设置 API 的类型。

- `OPEN_AI`: 使用 OpenAI API。
- `AZURE`: 使用 Azure API。
