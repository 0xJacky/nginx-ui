# 配置模板

Nginx UI Template 提供了一种开箱即用的配置模板机制。在 NgxConfigEditor 中，我们设计了一个可视化界面，使用户能够方便地插入模板中的配置到当前的配置文件中。
在本篇指南中，我们将绍这种配置模板的文件格式和语法规则。
配置模板文件存储在 `template/block` 目录中，我们欢迎并期待您通过提交 [PR](https://github.com/0xJacky/nginx-ui/pulls) 的形式分享您编写的配置模板。

::: tip
请注意，每次修改或添加新的配置文件后，需要重新编译后端以生效。
:::

## 文件格式

Nginx UI Template 文件由两部分组成：文件头部以及具体的 Nginx 配置。

以下是一个关于防盗链的配置模板，我们将以这个模板为基础为您介绍 Nginx UI Template 的文件格式及相关语法。

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

## 文件头部

文件头部应该包含在 `# Nginx UI Template Start` 和 `# Nginx UI Template End` 之间，遵循 toml 语法。

文件头部包含以下字段：

|           字段           |               描述               |             类型              | 必要 |
|:----------------------:|:------------------------------:|:---------------------------:|:--:|
|         `name`         |             配置的名称              |           string            | 是  |
|        `author`        |               作者               |           string            | 是  |
|     `description`      |    描述，使用 toml 格式的字典来实现多语言描述    |           toml 字典           | 是  |
| `variables.变量名称.type`  | 变量类型，目前支持 `boolean` 和 `string` |   string (boolean/string)   | 否  |
| `variables.变量名称.name`  | 变量显示的名称，是一个 toml 格式的字典，用于支持多语言 |           toml 字典           | 否  |
| `variables.变量名称.value` |             变量的默认值             | boolean/string (根据 type 定义) | 否  |

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

其中，名称、作者及描述将会以摘要的形式在配置列表中显示。

![配置列表](/assets/nginx-ui-template/zh_CN/config-template-list.png)

当您点击「查看」按钮，界面会显示一个对话框，如下图所示。

<img src="/assets/nginx-ui-template/zh_CN/config-ui.png" width="350px" title="配置 Modal" />

界面中的输入框和开关对应着变量的类型 `boolean` 和 `string`。

## Nginx 配置
Nginx 配置应该在文件头部之后提供，这部分将使用 Go 的 `text/template` 库进行解析。这个库提供了强大的模板生成能力，包括条件判断、循环以及复杂的文本处理等。
具体语法可以参考 [Go 文档](https://pkg.go.dev/text/template)。

在头部中定义的变量可以在这部分中使用，如 `.NoneReferer` 和 `.AllowReferers`。请注意，需要预先在头部定义变量，才能在这部分中使用。

示例如下：

```nginx configuration
location ~ .*\.(jpg|png|js|css)$ {
    valid_referers {{- if .NoneReferer}} none {{- end}} blocked server_names {{if .AllowReferers}}{{.AllowReferers}}{{- end}};
    if ($invalid_referer) {
        return 403;
    }
}
```

当用户在前端的输入框中输入变量的值后，系统将会自动生成新的配置内容，效果如下：
<img src="/assets/nginx-ui-template/zh_CN/config-ui-after-input.png" width="350px" title="配置 Modal" />

除了模板头部定义的变量，我们还提供了宏定义的变量，如下表所示：

|    变量名     |           描述            |
|:----------:|:-----------------------:|
|  HTTPPORT  |     Nginx UI 监听的端口      |
| HTTP01PORT | 用于 HTTP01 Challenge 的端口 |

上述变量可以直接在配置部分使用，无需在头部定义。
