---
outline: [2, 3]
---

# Templates and Translations

A plugin can contribute content instead of, or next to, code: nginx
configuration templates and translation files. It declares them in the
`content` block of its manifest, as directories in the package:

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

Content is data: nothing in it runs as a program. A plugin with only
`content` has no process; enabling and disabling it adds and removes its
contributions. Such a plugin declares no `capabilities`, `cron` or `events`,
since those are served by a process. Content may also sit next to `server`
and `webapp`.

## Templates

`content.templates` is a directory with up to two subdirectories, matching the
two lists of built-in templates:

- `conf/` for templates of a whole server;
- `block/` for snippets a person adds to a server.

At least one of them holds a template. A template is a file named like
`^[A-Za-z0-9][A-Za-z0-9._-]{0,63}\.conf$`; other entries are ignored and the
linter warns about them.

### Format

Templates use the format of the built-in ones:

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

- The header between `# Nginx UI Template Start` and `# Nginx UI Template End`
  is TOML with `name`, `author`, `description` (texts by language) and
  `variables`. Give every template a `name`; without it the file name is
  shown.
- Each variable has a `type` (`string`, `boolean` or `select`), a `name` by
  language, a default `value` and, for `select`, a `mask` that maps every
  choice to its labels.
- Everything after the end line is the body: a Go
  [text/template](https://pkg.go.dev/text/template) producing nginx directives
  valid inside a `server` block. An optional section between
  `# Nginx UI Custom Start` and `# Nginx UI Custom End` is rendered the same
  way and added to the server's custom directives.
- The template data is every variable under its key, plus `HTTPPORT` and
  `HTTP01PORT`, the ports Nginx UI listens on.

::: v-pre
Templates may use `{{.name}}`, `if`, `else` and `with`, and the functions
`and`, `or`, `not`, `eq`, `ne`, `lt`, `le`, `gt`, `ge`, `len`, `index`,
`print`, `println`, `html`, `js` and `urlquery`. Anything else (`range`,
`define`, `template`, `block`, `printf`, `call`) is refused, since loops and
format widths can make rendering unbounded. A template file is limited to 256
KiB and a rendered section to 1 MiB.
:::

### Validation

Nginx UI validates every template before it installs or updates the plugin,
and refuses the package when one lacks the start or end line, has an invalid
header, is not a valid template, fails to render with its default values or
renders to invalid nginx syntax.

Plugin templates appear in the same lists as the built-in ones, marked with
the plugin they come from. They never replace a built-in template or one of
another plugin, and disappear when the plugin is disabled. Their directives
are added to a configuration only when a person picks the template, after
seeing what it renders to.

## Translations

`content.locales` is a directory of GNU gettext PO files, one per language,
named `<lang>.po`, where `<lang>` is a language code of the Nginx UI
interface, such as `zh_CN` or `de_DE`. Provide only the languages you
translate. The directory holds at least one such file; other entries are
ignored and the linter warns about them. A file of a language a Nginx UI does
not have is not used there, and [`nginx-ui plugin lint`](../rules.md#content-locales)
warns about it; a file whose name is not a language code is an error.

A file is UTF-8, starts with the header entry (`msgid ""`) and has a
`msgstr` for every `msgid`. Entries marked `fuzzy` are ignored. Nginx UI
refuses a package with a file that does not parse.

While the plugin is enabled, its entries join the translations Nginx UI
serves for that language, so text the plugin shows through the gettext of
Nginx UI (its browser bundle, the names in its templates) is translated
without further work. A plugin cannot change a translation of Nginx UI
itself: when Nginx UI already translates a text, its translation wins. When
several plugins translate the same text, the plugin with the lowest id wins.

Translations replace display text only and are always treated as text,
never as markup.
