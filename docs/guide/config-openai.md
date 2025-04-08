# Open AI

This section is for setting up ChatGPT configurations. Please be aware that we do not check the accuracy of the
information you provide. If the configuration is incorrect, it might cause API request failures, making the ChatGPT
assistant unusable.

## BaseUrl

- Type: `string`

This option is used to set the base url of the api of Open AI, leave it blank if you do not need to change the url.

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

This option is used to set the ChatGPT model. If your account has the privilege to access the gpt-4 model, you can
configure this option accordingly.

## APIType

- Type: `string`
- Default: `OPEN_AI`

This option is used to set the type of the API.

- `OPEN_AI`: Use the OpenAI API.
- `AZURE`: Use the Azure API.
