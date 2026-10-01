---
outline: [2, 3]
---

# Slots

Slots are extension points of the Nginx UI interface. A bundle mounts a
component into one with `registry.registerSlot`:

```ts
registry.registerSlot('certificate.issue.footer', IssueHint, {
  order: 10,
  when: ctx => ctx.options.challenge_method === 'dns01',
})
```

| Option | Meaning |
| --- | --- |
| `order` | Lower values render first when several plugins use the same slot. |
| `when(ctx)` | Return `false` to leave the slot out for this context. |
| `label` | Display text for the slots that show one, an English text translated with the plugin's translations. |
| `sortValue`, `filters` | Sorting and filtering of log list columns. |

## Props

A mounted component receives the context of the slot twice: spread into
individual props and as one `context` prop. Both of these work:

```ts
defineProps<{ options: CertificateOptions }>()
defineProps<{ context: { options: CertificateOptions } }>()
```

Each mounted component has its own error boundary. A component that throws
shows an inline error instead of breaking the page or the other components
in the slot.

## Available Slots

| Slot | Context | Where |
| --- | --- | --- |
| `certificate.challenge.form:{method}` | `{ options }` | The settings of one ACME challenge method, e.g. `dns01`. |
| `certificate.issue.footer` | `{ options }` | The bottom of the certificate issue form. |
| `dns.credential.form:{provider_code}` | `{ credential, provider }` | Extra fields on the DNS credential form of one provider. |
| `dns.credential.hint:{provider_code}` | `{ credential, provider }` | Content above a provider's credential fields. |
| `plugin.settings:{plugin_id}` | `{ settings }` | Inside another plugin's settings. |
| `sidebar.footer` | `{}` | The bottom of the sidebar. |
| `nginx_log.view:{key}` | `{ path, type }` | An extra view of one log file. |
| `nginx_log.list.toolbar` | `{ type }` | The actions above the log list. |
| `nginx_log.list.column:{key}` | `{ row }` | An extra column of the log list. |
| `nginx_log.list.row.actions` | `{ row }` | The actions of one row of the log list. |
| `site.log.actions` | see [Site Log Actions](#site-log-actions) | The log actions of one site. |

A registration for a slot name Nginx UI does not know is ignored, and new
slots may appear in later versions. A slot a plugin defines for its own use
carries the plugin id as a prefix, see [Naming](./naming.md#settings-keys-and-slot-names).

### Certificate Challenge Form

`certificate.challenge.form:{method}` receives `options`, the reactive options
of the certificate being issued. A component reads and writes
`options.challenge_config`, an object that belongs to the plugin: Nginx UI
stores it with the certificate and hands it unchanged to the plugin in every
DNS-01 call as `options`. See [DNS-01](./capabilities/dns01.md#certificate-options).

## Log Page Slots

### Views

Each `nginx_log.view:{key}` registration adds a mode to the switch of the log
page, next to the built-in raw view, labeled with `opts.label` (the key when
there is none). The page keeps the chosen mode in its `view` query parameter,
so `?view=<key>` links straight to it. Only the chosen component is mounted,
with the `path` and `type` (`access` or `error`) of the file. `opts.when(ctx)`
decides whether the mode is offered for a file.

### Columns

Each `nginx_log.list.column:{key}` registration adds a column after the
built-in ones, ordered by `opts.order`, titled with `opts.label`. The
component renders the cell of the row it receives.

`opts.when(ctx)` is called once per list with `{ type }`, not per row, and
decides whether the column exists at all. Register an access log only column
once and hide it on the error list:

```ts
registry.registerSlot('nginx_log.list.column:index_status', StatusCell, {
  label: 'Index Status',
  when: ctx => ctx.type === 'access',
  sortValue: row => statusRank(row.path),
  filters: [{ label: 'Indexed', value: 'indexed', match: row => isIndexed(row.path) }],
})
```

Nginx UI loads the whole list at once and sorts and filters plugin columns in
the browser:

- With `sortValue(row)` the header sorts by the returned value: numbers by
  size, strings in locale order, `null` and `undefined` last.
- With `filters` the header offers one choice per entry, labeled with
  `label`. Selected choices of one column combine with OR, columns with AND.
  `value` identifies the choice.

A row carries at least `path`, `type`, `name` and `config_file`. Ignore
fields you do not know.

### Toolbar and Row Actions

`nginx_log.list.toolbar` and `nginx_log.list.row.actions` render every
registration in `opts.order`. `opts.when` receives the same context as the
component.

### Site Log Actions

`site.log.actions` appears in the site editor and the site list and receives
the log files of one site:

| Prop | Meaning |
| --- | --- |
| `siteName` | The site. |
| `accessLogPath`, `errorLogPath` | The site's own `access_log` or `error_log` path, else the nginx default log it falls back to, else an empty string. |
| `accessLogInherited`, `errorLogInherited` | `true` when the path is the nginx default log rather than the site's own. |

A default log may hold the traffic of other sites. An action about one site's
traffic should hide itself, or say so, when the path is inherited.
