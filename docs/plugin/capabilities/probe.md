---
outline: [2, 3]
---

# Health Checks

A `probe` plugin adds ways to check whether a site is healthy, beyond the
built-in HTTP and gRPC checks: a TCP banner, a database login, a status API.
Its kinds appear as choices in a site's health check, and Nginx UI runs the
chosen kind on the check's schedule.

| Method | Required | Purpose |
| --- | --- | --- |
| `probe.check` | yes | Check one target once. |

A `probe` plugin needs no permission to receive calls, but should request
`network` since it reaches out to the target.

## Declaring Kinds

```json [plugin.json]
"capabilities": ["probe"],
"permissions": ["network"],
"probe": {
  "kinds": [
    {
      "code": "tcp-banner",
      "name": "TCP banner",
      "configuration": {
        "fields": [
          { "key": "port", "type": "number", "display_name": "Port", "required": true },
          { "key": "expect", "display_name": "Expected banner prefix", "help_text": "For example SSH-2.0" }
        ]
      }
    }
  ]
}
```

| Field | Required | Meaning |
| --- | --- | --- |
| `code` | yes | Identifier of the kind, see [Naming](../naming.md#provider-and-kind-codes). Unique in the manifest. |
| `name` | yes | Name of the kind. |
| `configuration.fields` | no | The fields of the check form, see [Configuration Form](./notify.md#configuration-form). |

## Checking

```json
{
  "jsonrpc": "2.0", "id": 34, "method": "probe.check",
  "params": { "kind": "tcp-banner", "target": "https://example.com", "config": { "port": "22", "expect": "SSH-2.0" }, "timeout_seconds": 10 }
}
```

| Field | Meaning |
| --- | --- |
| `kind` | The `code` of the kind. |
| `target` | What to check: the site URL, or the custom target of the health check. A kind uses the parts it needs, a TCP check only the host name. |
| `config` | The values of the check form. Treat them as secret. |
| `timeout_seconds` | How long Nginx UI waits. Finish within it. |

```json
{ "jsonrpc": "2.0", "id": 34, "result": { "status": "down", "latency_ms": 10000, "message": "no banner within 10s" } }
```

| Field | Meaning |
| --- | --- |
| `status` | `up`, `down` or `degraded`. |
| `latency_ms` | How long the check took, not negative. |
| `message` | Detail shown to the person, expected whenever `status` is not `up`. Never a credential. |

`degraded` means the target answers, but not as well as it should: slowly,
partly failing, or with a warning. A target that is down or does not answer
in time is a **result**: report `down` with a message. Answer an error only
when the check itself cannot run: `-32003` with `data.field` for a
configuration it cannot use, `-32000` otherwise.

Nginx UI shows a failed call (an error, a timeout, a missing plugin)
differently from `down`, so the person can tell a down target from a check
that did not run. It counts `degraded` as online with a message and allows
five seconds beyond `timeout_seconds`.
