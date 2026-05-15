# Open AI

This section is for setting up ChatGPT configurations. Please be aware that we do not check the accuracy of the
information you provide. If the configuration is incorrect, it might cause API request failures, making the ChatGPT
assistant unusable.

## Provider

- Type: `string`
- Default: `openai`

This option selects a preset for OpenAI-compatible providers.

- `openai`: Use the default OpenAI endpoint.
- `atlas_cloud`: Use the Atlas Cloud endpoint `https://api.atlascloud.ai/v1`.
- `custom`: Keep using the custom `BaseUrl` value.

## BaseUrl

- Type: `string`

This option is used to set the base URL of the API. Leave it blank if you do not need to change the URL.

For Atlas Cloud, use `https://api.atlascloud.ai/v1`. Atlas Cloud is OpenAI-compatible, so the existing chat and code
completion features work without additional backend changes. You can find the Atlas Cloud model guide at
<https://www.atlascloud.ai/docs/models/get-start>.

## Token

- Type: `string`

This option is used to set the token of the api of Open AI.

## Proxy

- Type: `string`

This option is used to configure the proxy for OpenAI's API. If you are unable to access OpenAI's API in your country or
region, you can use an HTTP proxy and set this option to the corresponding URL.

## Model

- Type: `string`
- Default: `gpt-3.5-turbo`

This option is used to set the chat model. If your account has the privilege to access the gpt-4 model, you can
configure this option accordingly.

## APIType

- Type: `string`
- Default: `OPEN_AI`

This option is used to set the type of the API.

- `OPEN_AI`: Use the OpenAI API.
- `AZURE`: Use the Azure API.

## EnableCodeCompletion

- Type: `boolean`
- Default: `false`
- Version: `>=2.0.0-rc.6`

This option is used to enable the code completion feature in the code editor.

## CodeCompletionModel

- Type: `string`
- Version: `>=2.0.0-rc.6`

This option is used to set the code completion model, leave it blank if you want to use the chat model.
