# Config Template

Nginx UI Template provides out-of-the-box configuration templates for users. In `NgxConfigEditor`, we offer a UI where users can quickly insert configurations from the template into the current configuration file.
In this document, we will describe the file format and syntax of it.

The configuration templates are stored in `template/block`, and we welcome you to share your own configuration templates by open a [PR](https://github.com/0xJacky/nginx-ui/pulls).

::: tip
Please note, you need to recompile the backend after modifying or adding new configuration files.
:::

## File Format

Nginx UI Template file consists of two parts: the file header and the actual Nginx configuration.

Below is a configuration template for hotlink protection, which we will use as a basis to introduce the file format and related syntax of Nginx UI Template.

```nginx configuration
# Nginx UI Template Start
name = "Hotlink Protection"
author = "@0xJacky"
description = { en = "Hotlink Protection Config Template", zh_CN = "防盗链配置模板"}

[variables.NoneReferer]
type = "boolean"
name = { en = "Allow Referer is None", zh_CN = "允许空 Referer"}
value = false

[variables.AllowReferers]
type = "string"
name = { en = "Allow Referers", zh_CN = "允许的 Referers"}
value = ""
# Nginx UI Template End

location ~ .*\.(jpg|png|js|css)$ {
    valid_referers {{- if .NoneReferer}} none {{- end}} blocked server_names {{if .AllowReferers}}{{.AllowReferers}}{{- end}};
    if ($invalid_referer) {
        return 403;
    }
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
| `variables.VariableName.type`  |       Variable type, currently supports `boolean` and `string`        |                    string                     |    No    |
| `variables.VariableName.name`  | Variable display name, is a toml dictionary to support multi-language |                toml dictionary                |    No    |
| `variables.VariableName.value` |                     Default value of the variable                     | boolean/string (according to type definition) |    No    |

示例如下：

```toml
# Nginx UI Template Start
name = "Hotlink Protection"
author = "@0xJacky"
description = { en = "Hotlink Protection Config Template", zh_CN = "防盗链配置模板"}

[variables.NoneReferer]
type = "boolean"
name = { en = "Allow Referer is None", zh_CN = "允许空 Referer"}
value = false

[variables.AllowReferers]
type = "string"
name = { en = "Allow Referers", zh_CN = "允许的 Referers"}
value = ""
# Nginx UI Template End
```

The name, author, and description will be displayed in the configuration list as a summary.

![Config template list](/assets/nginx-ui-template/en/config-template-list.png)

When you click the "View" button, a dialog will appear, as shown below.

<img src="/assets/nginx-ui-template/en/config-ui.png" width="350px" title="Config Modal" />

The input boxes and switches in the interface correspond to the variable types `boolean` and `string`.

## Nginx Configuration
The Nginx configuration should be provided after the file header. This part will be parsed using the Go `text/template` library. This library provides powerful template generation capabilities, including conditional judgment, looping, and complex text processing, etc.
For more information, please check [Go Documentation](https://pkg.go.dev/text/template).

The variables defined in the header can be used in this part, such as `.NoneReferer` and `.AllowReferers`.
Please note that you need to define the variables in the header in advance before using them in this part.

Here is an example:

```nginx configuration
location ~ .*\.(jpg|png|js|css)$ {
    valid_referers {{- if .NoneReferer}} none {{- end}} blocked server_names {{if .AllowReferers}}{{.AllowReferers}}{{- end}};
    if ($invalid_referer) {
        return 403;
    }
}
```

When users input variable values in the frontend input boxes, the system will automatically generate new configuration content, as shown below:

<img src="/assets/nginx-ui-template/en/config-ui-after-input.png" width="350px" title="Config Modal" />

In addition to the variables defined in the template header, we also provide macro-defined variables, as shown in the table below:

| Variable Name |        Description        |
|:-------------:|:-------------------------:|
|   HTTPPORT    |  Nginx UI listening port  |
|  HTTP01PORT   | Port for HTTP01 Challenge |

The variables above can be used directly in the configuration part without definition in the header.
