# Open AI

本節用於設定 ChatGPT 設定。請注意，我們不會檢查您提供的資訊的準確性。如果設定錯誤，可能會導致 API 請求失敗，導致 ChatGPT
助手無法使用。

## Provider

- 型別：`string`
- 預設值：`openai`

此選項用於選擇 OpenAI 相容服務商預設。

- `openai`：使用預設 OpenAI 端點。
- `atlas_cloud`：使用 Atlas Cloud 端點 `https://api.atlascloud.ai/v1`。
- `minimax`：使用 MiniMax 全球 OpenAI 相容端點 `https://api.minimax.io/v1`。
- `custom`：繼續使用自訂 `BaseUrl` 值。

## BaseUrl

- 型別：`string`

此選項用於設定 Open AI API 的基本 URL，如果不需要更改 URL，則將其保留為空。

Atlas Cloud 使用 `https://api.atlascloud.ai/v1`。

MiniMax 的 OpenAI 相容端點包括全球服務 `https://api.minimax.io/v1` 和中國服務
`https://api.minimaxi.com/v1`。MiniMax 也提供 Anthropic 相容端點
`https://api.minimax.io/anthropic/v1` 和 `https://api.minimaxi.com/anthropic/v1`；此設定應使用 OpenAI 相容端點。
建議的 MiniMax 文字模型包括 `MiniMax-M3` 和 `MiniMax-M2.7`。文件見 <https://platform.minimax.io/docs> 和
<https://platform.minimaxi.com/docs>。

## Token

- 型別：`string`

此選項用於設定 Open AI API 的令牌。

## Proxy

- 型別：`string`

此選項用於為 OpenAI 的 API 設定代理。如果您在國家或地區無法存取 OpenAI 的 API，可以使用 HTTP 代理並將此選項設定為相應的
URL。

## Model

- 型別：`string`
- 預設值：`gpt-3.5-turbo`

此選項用於設定對話模型。如果您的帳戶有許可權訪問 `gpt-4` 模型，可以相應地配置此選項。

## APIType

- 型別：`string`
- 預設值：`OPEN_AI`

此選項用於設定 API 的類型。

- `OPEN_AI`: 使用 OpenAI API。
- `AZURE`: 使用 Azure API。

## EnableCodeCompletion

- 型別：`boolean`
- 預設值：`false`
- 版本：`>=2.0.0-rc.6`

此選項用於啟用編輯器代碼補全功能。

## CodeCompletionModel

- 型別：`string`
- 版本：`>=2.0.0-rc.6`

此選項用於設定代碼補全的模型，留空則使用對話模型。
