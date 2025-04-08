# Open AI

本節用於設定 ChatGPT 配置。請注意，我們不會檢查您提供的資訊的準確性。如果配置錯誤，可能會導致 API 請求失敗，導致 ChatGPT
助手無法使用。

## BaseUrl

- 型別：`string`

此選項用於設定 Open AI API 的基本 URL，如果不需要更改 URL，則將其保留為空。

## Token

- 型別：`string`

此選項用於設定 Open AI API 的令牌。

## Proxy

- 型別：`string`

此選項用於為 OpenAI 的 API 配置代理。如果您在國家或地區無法訪問 OpenAI 的 API，可以使用 HTTP 代理並將此選項設定為相應的
URL。

## Model

- 型別：`string`
- 預設值：`gpt-3.5-turbo`

此選項用於設定 ChatGPT 模型。如果您的帳戶有許可權訪問 `gpt-4` 模型，可以相應地配置此選項。

## APIType

- 型別：`string`
- 預設值：`OPEN_AI`

此選項用於設定 API 的類型。

- `OPEN_AI`: 使用 OpenAI API。
- `AZURE`: 使用 Azure API。
