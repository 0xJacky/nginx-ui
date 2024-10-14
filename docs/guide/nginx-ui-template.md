# Config Template

Nginx UI Template provides out-of-the-box configuration templates for users. In `NgxConfigEditor`, we offer a UI where users can quickly insert configurations from the template into the current configuration file.
In this document, we will describe the file format and syntax of it.

The configuration templates are stored in `template/block`, and we welcome you to share your own configuration templates by open a [PR](https://github.com/0xJacky/nginx-ui/pulls).

::: tip
Please note, you need to recompile the backend after modifying or adding new configuration files.
:::

## File Format

Nginx UI Template file consists of two parts: the file header and the actual Nginx configuration.

Below is a configuration template for reverse proxy, which we will use as a basis to introduce the file format and related syntax of Nginx UI Template.

```nginx configuration
# Nginx UI Template Start
name = "Reverse Proxy"
author = "@0xJacky"
description = { en = "Reverse Proxy Config", zh_CN = "反向代理配置"}

[variables.enableWebSocket]
type = "boolean"
name = { en = "Enable WebSocket", zh_CN = "启用 WebSocket"}
value = true

[variables.clientMaxBodySize]
type = "string"
name = { en = "Client Max Body Size", zh_CN = "客户端最大请求内容大小"}
value = "1000m"

[variables.scheme]
type = "select"
name = { en = "Scheme", zh_CN = "协议"}
value = "http"
mask = { http = { en = "HTTP" }, https = { en = "HTTPS" } }

[variables.host]
type = "string"
name = { en = "Host", zh_CN = "主机"}
value = "127.0.0.1"

[variables.port]
type = "string"
name = { en = "Port", zh_CN = "端口"}
value = 9000
# Nginx UI Template End

# Nginx UI Custom Start
{{- if .enableWebSocket }}
map $http_upgrade $connection_upgrade {
    default upgrade;
    '' close;
}
{{- end }}
# Nginx UI Custom End

if ($host != $server_name) {
    return 404;
}

location / {
        {{ if .enableWebSocket }}
        proxy_http_version 1.1;
        proxy_set_header Upgrade $http_upgrade;
        proxy_set_header Connection $connection_upgrade;
        {{ end }}

        client_max_body_size {{ .clientMaxBodySize }};

        proxy_redirect off;
        proxy_set_header Host $host;
        proxy_set_header X-Real-IP $remote_addr;
        proxy_set_header X-Forwarded-For $proxy_add_x_forwarded_for;
        proxy_set_header X-Forwarded-Proto $scheme;

        proxy_pass {{ .scheme }}://{{ .host }}:{{ .port }}/;
 }
```

## File Header

The file header should be placed between `# Nginx UI Template Start` and `# Nginx UI Template End`, and should follow the toml syntax.

The file header includes the following fields:

|             Field              |                              Description                              |                     Type                      | Required |
|:------------------------------:|:---------------------------------------------------------------------:|:---------------------------------------------:|:--------:|
|             `name`             |                       Name of the configuration                       |                    string                     |   Yes    |
|            `author`            |                                Author                                 |                    string                     |   Yes    |
|         `description`          |     Desciption, uses a toml dictionary for multi-language support     |                toml dictionary                |   Yes    |
| `variables.VariableName.type`  |  Variable type, currently supports `boolean`, `string` and `select`   |                    string                     |   Yes    |
| `variables.VariableName.name`  | Variable display name, is a toml dictionary to support multi-language |                toml dictionary                |   Yes    |
| `variables.VariableName.value` |                     Default value of the variable                     | boolean/string (according to type definition) |    No    |
| `variables.VariableName.mask`  |                         The options of select                         |                toml dictionary                |    No    |

Example:

```toml
# Nginx UI Template Start
name = "Reverse Proxy"
author = "@0xJacky"
description = { en = "Reverse Proxy Config", zh_CN = "反向代理配置"}

[variables.enableWebSocket]
type = "boolean"
name = { en = "Enable WebSocket", zh_CN = "启用 WebSocket"}
value = true

[variables.clientMaxBodySize]
type = "string"
name = { en = "Client Max Body Size", zh_CN = "客户端最大请求内容大小"}
value = "1000m"

[variables.scheme]
type = "select"
name = { en = "Scheme", zh_CN = "协议"}
value = "http"
mask = { http = { en = "HTTP" }, https = { en = "HTTPS" } }

[variables.host]
type = "string"
name = { en = "Host", zh_CN = "主机"}
value = "127.0.0.1"

[variables.port]
type = "string"
name = { en = "Port", zh_CN = "端口"}
value = 9000
# Nginx UI Template End
```

The name, author, and description will be displayed in the configuration list as a summary.

![Config template list](/assets/nginx-ui-template/en/config-template-list.png)

When you click the "View" button, a dialog will appear, as shown below.

<img src="/assets/nginx-ui-template/en/config-ui.png" width="350px" title="Config Modal" />

The following table shows the relationship between the variable type and the UI element:

| Variable Type | UI Element |
|:-------------:|:----------:|
| `boolean`     | switcher   |
| `string`      | input      |
| `select`      | select     |

## Nginx Configuration
The Nginx configuration should be provided after the file header. This part will be parsed using the Go `text/template` library. This library provides powerful template generation capabilities, including conditional judgment, looping, and complex text processing, etc.
For more information, please check [Go Documentation](https://pkg.go.dev/text/template).

The variables defined in the header can be used in this part, such as `.scheme`, `.host` and `.port`.
Please note that you need to define the variables in the header in advance before using them in this part.

Here is an example:

```nginx configuration
location / {
        {{ if .enableWebSocket }}
        proxy_http_version 1.1;
        proxy_set_header Upgrade $http_upgrade;
        proxy_set_header Connection $connection_upgrade;
        {{ end }}

        client_max_body_size {{ .clientMaxBodySize }};

        proxy_redirect off;
        proxy_set_header Host $host;
        proxy_set_header X-Real-IP $remote_addr;
        proxy_set_header X-Forwarded-For $proxy_add_x_forwarded_for;
        proxy_set_header X-Forwarded-Proto $scheme;
        proxy_set_header Forwarded $proxy_add_forwarded;

        proxy_pass {{ .scheme }}://{{ .host }}:{{ .port }}/;
 }
```

When users change the form, the system will automatically generate new configuration content based on the template and the input of user.

In addition to the variables defined in the template header, we also provide macro-defined variables, as shown in the table below:

| Variable Name |        Description        |
|:-------------:|:-------------------------:|
|   HTTPPORT    |  Nginx UI listening port  |
|  HTTP01PORT   | Port for HTTP01 Challenge |

The variables above can be used directly in the configuration part without definition in the header.
