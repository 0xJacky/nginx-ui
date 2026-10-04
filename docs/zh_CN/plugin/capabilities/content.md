---
outline: [2, 3]
---

# 模板与翻译

插件可以提供内容来代替代码，或与代码一起提供：nginx 配置模板和翻译文件。内容在清单的 `content` 块中以插件包内目录的形式声明：

```json [plugin.json]
{
  "id": "io.github.example.snippets",
  "name": "Extra snippets",
  "version": "1.0.0",
  "api_version": 1,
  "content": {
    "templates": "templates",
    "locales": "locales"
  }
}
```

```text
io.github.example.snippets/
├── plugin.json
├── README.md
├── LICENSE
├── templates/
│   ├── block/
│   │   └── cache-static.conf
│   └── conf/
│       └── ghost.conf
└── locales/
    ├── de_DE.po
    └── zh_CN.po
```

内容就是数据：其中任何内容都不会作为程序运行。只有 `content` 的插件没有进程，启用和禁用它只会添加和移除它提供的内容。这类插件不能声明 `capabilities`、`cron` 或 `events`，因为它们由进程提供。内容也可以与 `server` 和 `webapp` 同时存在。

## 模板 {#templates}

`content.templates` 是一个最多包含两个子目录的目录，与内置模板的两个列表对应：

- `conf/` 用于整个 server 的模板；
- `block/` 用于用户添加到 server 中的片段。

其中至少一个包含模板。模板是文件名匹配 `^[A-Za-z0-9][A-Za-z0-9._-]{0,63}\.conf$` 的文件；其他条目会被忽略，检查工具会对它们发出警告。

### 格式 {#format}

模板使用与内置模板相同的格式：

```text
# Nginx UI Template Start
name = "Cache static files"
author = "@example"
description = { en = "Cache images, scripts and styles", de_DE = "Bilder, Skripte und Stylesheets zwischenspeichern" }

[variables.expires]
type = "string"
name = { en = "Expires", de_DE = "Ablauf" }
value = "30d"
# Nginx UI Template End
location ~* \.(?:css|js|png|jpe?g|gif|svg|webp)$ {
    expires {{.expires}};
    add_header Cache-Control "public";
}
```

- `# Nginx UI Template Start` 与 `# Nginx UI Template End` 之间的头部是 TOML，包含 `name`、`author`、`description`（各语言的文本）和 `variables`。请为每个模板设置 `name`，没有时会显示文件名。
- 每个变量有 `type`（`string`、`boolean` 或 `select`）、各语言的 `name`、默认值 `value`，`select` 类型还有把每个选项映射到其标签的 `mask`。
- 结束行之后的所有内容是模板主体：一个 Go [text/template](https://pkg.go.dev/text/template)，输出在 `server` 块中有效的 nginx 指令。`# Nginx UI Custom Start` 与 `# Nginx UI Custom End` 之间的可选部分会以同样方式渲染，并加入 server 的自定义指令。
- 模板数据是以键区分的每个变量，以及 `HTTPPORT` 和 `HTTP01PORT`，即 Nginx UI 监听的端口。

::: v-pre
模板可以使用 `{{.name}}`、`if`、`else` 和 `with`，以及函数 `and`、`or`、`not`、`eq`、`ne`、`lt`、`le`、`gt`、`ge`、`len`、`index`、`print`、`println`、`html`、`js` 和 `urlquery`。其他任何写法（`range`、`define`、`template`、`block`、`printf`、`call`）都会被拒绝，因为循环和格式宽度可能让渲染失去边界。模板文件最大 256 KiB，渲染后的每部分最大 1 MiB。
:::

### 校验 {#validation}

Nginx UI 在安装或更新插件前会校验每个模板。缺少开始或结束行、头部无效、不是有效的模板、用默认值渲染失败或渲染出无效 nginx 语法的模板，都会导致插件包被拒绝。

插件的模板与内置模板出现在同样的列表中，并标明来自哪个插件。它们永远不会替换内置模板或其他插件的模板，并在插件禁用后消失。只有用户选择某个模板并看到它渲染出的内容后，模板的指令才会加入配置。

## 翻译 {#translations}

`content.locales` 是一个包含 GNU gettext PO 文件的目录，每种语言一个文件，命名为 `<lang>.po`，其中 `<lang>` 是 Nginx UI 界面的语言代码，例如 `zh_CN` 或 `de_DE`。只需提供你翻译了的语言。目录至少包含一个这样的文件；其他条目会被忽略，检查工具会对它们发出警告。Nginx UI 没有的语言的文件在该 Nginx UI 上不会使用，[`nginx-ui plugin lint`](../rules.md#content-locales) 会对它发出警告；文件名不是语言代码时为错误。

文件使用 UTF-8，以头部条目（`msgid ""`）开始，每个 `msgid` 都有 `msgstr`。标记为 `fuzzy` 的条目会被忽略。包含无法解析的文件的插件包会被 Nginx UI 拒绝。

插件启用期间，它的条目会加入 Nginx UI 为该语言提供的翻译，因此插件通过 Nginx UI 的 gettext 显示的文字（浏览器包、模板中的名称）无需额外工作即可翻译。插件不能修改 Nginx UI 自身的翻译：Nginx UI 已翻译的文本以它的翻译为准。多个插件翻译同一段文本时，以插件 ID 最小的插件为准。

翻译只替换显示文本，并且始终被当作文本处理，绝不会当作标记。
