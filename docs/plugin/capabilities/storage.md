---
outline: [2, 3]
---

# Storage

A `storage` plugin keeps files in places Nginx UI cannot reach on its own: a
WebDAV share, an SFTP server, a cloud drive. Its backends appear next to the
built-in storage, and Nginx UI uses them as destinations of automatic
backups.

| Method | Required | Purpose |
| --- | --- | --- |
| `storage.put` | yes | Store a file under a key. |
| `storage.get` | yes | Fetch the object under a key into a file. |
| `storage.list` | yes | List the objects whose key starts with a prefix. |
| `storage.delete` | yes | Remove the object under a key. |
| `storage.validate` | no | Check a configuration without storing anything. |

File contents never travel in a message. Nginx UI and the plugin exchange
them as files, so a backup of several gigabytes costs neither side a copy in
memory. The plugin should request `network`.

## Declaring Backends

```json [plugin.json]
"capabilities": ["storage"],
"permissions": ["network"],
"storage": {
  "backends": [
    {
      "code": "webdav",
      "name": "WebDAV",
      "configuration": {
        "fields": [
          { "key": "url", "display_name": "Server URL", "help_text": "Folder the objects are kept in", "required": true },
          { "key": "username", "display_name": "Username", "required": true },
          { "key": "password", "display_name": "Password", "required": true, "secret": true }
        ]
      }
    }
  ]
}
```

`code`, `name` and `configuration.fields` work as for
[notification channels](./notify.md#configuration-form).

## Keys

A key names one object. It is a relative path of `/` separated segments, at
most 1024 bytes, and never empty, never starting or ending with `/`, without
empty, `.` or `..` segments, `\` or control characters. Keys are case
sensitive UTF-8 and may contain letters of any script. Map them onto the
service's own naming as needed (for example by percent-encoding) and return
them unchanged from `storage.list`. Answer `-32602` to a key of another form.

## Exchanging Files

Files pass through the `exchange` directory of the plugin's data directory.
For every call that moves a file, Nginx UI creates a fresh subdirectory with
a random name and passes absolute paths inside it:

- For `storage.put`, the file is at `source_path` before the call. Only read
  it: do not change, move or delete it.
- For `storage.get`, `target_path` does not exist yet. Create it as a regular
  file with the whole object, and remove a partial file when you fail.

::: warning
Do not touch any other path of the exchange directory, and do not keep or use
a path after answering: Nginx UI removes the subdirectory once the call
returns. A plugin that cannot open or create the file answers `-32000`.
:::

## Storing

```json
{
  "jsonrpc": "2.0", "id": 42, "method": "storage.put",
  "params": {
    "backend": "webdav",
    "config": { "url": "https://dav.example/remote.php/dav/files/alice", "username": "alice", "password": "app-password-xxx" },
    "key": "nginx-ui/daily_1790000000.zip",
    "source_path": "/var/lib/nginx-ui/plugins/.data/io.github.example.webdav/exchange/5f0c2a9d/daily_1790000000.zip"
  }
}
```

Store the whole file under `key`, replacing an existing object, and answer
only once it is durable: `{ "size": <bytes stored> }`. A failed put should not
leave a partial object behind. Errors are `-32003` with `data.field` for
causes in the configuration (a wrong password, a missing folder) and `-32000`
for everything else (an outage, a full disk, a network failure).

## Fetching

`storage.get` has `backend`, `config`, `key` and `target_path`. Write the whole
object to `target_path` and answer `{ "size": <bytes written> }`. A key without
an object is `-32000` with a message saying so.

## Listing

::: code-group

```json [Request]
{ "jsonrpc": "2.0", "id": 44, "method": "storage.list", "params": { "backend": "webdav", "config": {}, "prefix": "nginx-ui/daily_" } }
```

```json [Response]
{ "jsonrpc": "2.0", "id": 44, "result": { "objects": [
  { "key": "nginx-ui/daily_1790000000.zip", "size": 5242880, "modified_at": "2026-09-21T03:00:05Z" }
] } }
```
:::

`prefix` is a plain string prefix, not a directory: `nginx-ui/daily_` matches
`nginx-ui/daily_1.zip`, and an empty prefix lists everything. List every
match, following the service's pagination, in any order. `modified_at` is an
RFC 3339 time, empty when the service does not tell. Leave out objects you
did not store when you can tell them apart. No match is an empty list.

## Deleting

`storage.delete` has `backend`, `config` and `key`. Remove the object and
answer `{}`. Deleting a key without an object also succeeds, so Nginx UI can
retry a cleanup.

## Validating

`storage.validate` has `backend` and `config`. Check the configuration without
storing, changing or removing anything, and preferably without contacting the
service. Answer `-32003` with `data.field` for the first problem, `{}`
otherwise. To find out whether the service accepts the configuration,
Nginx UI calls `storage.list`.

Never write a `config` value or the content of a file to a log.

## How Backups Use It

- The storage path of a backup task is the key prefix. Each run stores
  `<prefix>/<name>_<unix time>.zip`, plus `<prefix>/<name>_<unix time>.zip.key`
  for an encrypted backup.
- A task that keeps only its latest backups calls `storage.list` with its
  prefix after each run and `storage.delete` for the older ones.
- Restoring calls `storage.get` for the archive and its key file.
- The test button of the backup form calls `storage.validate`, then
  `storage.list`.

Nginx UI waits 10 minutes for `storage.put` and `storage.get` and 30 seconds
for the other methods, and retries nothing on its own.
