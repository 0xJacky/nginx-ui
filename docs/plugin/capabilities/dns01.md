---
outline: [2, 3]
---

# DNS-01

A `dns01` plugin solves the ACME DNS-01 challenge for a DNS provider: it
publishes the `_acme-challenge` TXT record through the provider's API while a
certificate is issued, and removes it afterwards. Its providers appear in the
certificate form and in the DNS credential editor of Nginx UI.

| Method | Required | Purpose |
| --- | --- | --- |
| `dns01.present` | yes | Publish the challenge TXT record. |
| `dns01.cleanup` | yes | Remove what `present` published. |
| `dns01.validate` | no | Check a credential without publishing anything. |
| `dns01.options` | no | Report the propagation timing of a provider. |
| `dns01.check` | no | Report whether the record has propagated. |

An optional method the plugin does not implement answers `-32002`
(Unsupported), and Nginx UI falls back to its own behavior.

## Declaring Providers

```json [plugin.json]
"capabilities": ["dns01"],
"permissions": ["network"],
"dns01": {
  "providers": [
    {
      "name": "MyDNS",
      "code": "mydns",
      "links": { "api": "https://mydns.example/docs/api" },
      "propagation_timeout_seconds": 120,
      "polling_interval_seconds": 2,
      "form": {
        "fields": [
          { "key": "MYDNS_API_TOKEN", "label": "API token", "group": "credential", "secret": true },
          { "key": "MYDNS_TTL", "label": "TXT record TTL", "group": "setting", "default": "120", "unit": "seconds" }
        ]
      }
    }
  ]
}
```

| Field | Required | Meaning |
| --- | --- | --- |
| `name` | yes | Name of the DNS provider. |
| `code` | yes | Identifier of the provider, see [Naming](../naming.md#provider-and-kind-codes). Unique in the manifest. |
| `links.api` | no | Link to the provider's API documentation. |
| `propagation_timeout_seconds` | no | How long Nginx UI waits for the record to propagate. |
| `polling_interval_seconds` | no | How often propagation is checked. |
| `form` | yes | The values the provider takes. See [Credential Form](#credential-form). |

## Publishing the Record

```json
{
  "jsonrpc": "2.0", "id": 10, "method": "dns01.present",
  "params": {
    "provider": "mydns",
    "config": { "MYDNS_API_TOKEN": "tok_live_xxx", "MYDNS_TTL": "120" },
    "options": { "credential_id": "7", "disable_cname": false },
    "domain": "example.com",
    "fqdn": "_acme-challenge.example.com.",
    "effective_fqdn": "_acme-challenge.example.com.",
    "value": "gfj9Xq...Rg85nM",
    "token": "evaGxfADs...62jcerQ",
    "key_auth": "evaGxfADs...62jcerQ.9jg46WB3...GKl3Z7",
    "dry_run": false
  }
}
```

| Field | Meaning |
| --- | --- |
| `provider` | The `code` of the provider. |
| `config` | The values of the credential: both groups of the form merged into one map, keyed by field `key`. |
| `options` | The certificate's options from the challenge form. See [Certificate Options](#certificate-options). |
| `domain` | The domain being validated, without a leading `*.`. |
| `fqdn` | The `_acme-challenge` name. |
| `effective_fqdn` | Where the record goes: the target of a CNAME when one was followed, otherwise `fqdn`. |
| `value` | The TXT record value. |
| `token`, `key_auth` | The ACME challenge token and key authorization, for a plugin that derives `value` itself. |
| `dry_run` | When `true`, check the input only. Do not contact the provider or change any record. |

Answer `{}` once the record is published. On failure answer an error:

- `-32003` with `data.field` naming the field when the cause is something the
  person entered, such as a rejected token;
- `-32000` for anything else, such as an outage of the provider.

::: warning
Never let a `config` value appear in a log line or an error message.
:::

## Removing the Record

`dns01.cleanup` takes the same parameters as `dns01.present` and removes the
record published for the same `effective_fqdn` and `value`, answering `{}`.
Nginx UI calls it unconditionally when an issuance ends, so it must succeed
when there is nothing to remove, including when `present` never ran or
failed.

## Validating a Credential

```json
{ "jsonrpc": "2.0", "id": 11, "method": "dns01.validate", "params": { "provider": "mydns", "config": { "MYDNS_API_TOKEN": "" } } }
```

Check the values without contacting the provider: answer `-32003` with
`data.field` for the first missing or malformed field and `{}` when they look
usable. Nginx UI calls it while a person fills in the credential form.

## Propagation Timing

::: code-group

```json [Request]
{ "jsonrpc": "2.0", "id": 12, "method": "dns01.options", "params": { "provider": "mydns", "config": {}, "options": {} } }
```

```json [Response]
{ "jsonrpc": "2.0", "id": 12, "result": { "propagation_timeout_seconds": 120, "polling_interval_seconds": 2, "sequential_interval_seconds": 0 } }
```
:::

`sequential_interval_seconds`, when not `0`, tells Nginx UI that the provider
cannot handle two challenges of one account at the same time, and that calls
must be at least that many seconds apart. Without this method Nginx UI uses
the timing in the manifest, then its own defaults.

## Checking Propagation

::: code-group

```json [Request]
{
  "jsonrpc": "2.0", "id": 13, "method": "dns01.check",
  "params": { "provider": "mydns", "config": {}, "options": {}, "domain": "example.com", "fqdn": "_acme-challenge.example.com.", "value": "gfj9Xq...Rg85nM", "key_auth": "evaGxfADs...62jcerQ.9jg46WB3...GKl3Z7" }
}
```

```json [Response]
{ "jsonrpc": "2.0", "id": 13, "result": { "ready": false, "effective_fqdn": "_acme-challenge.example.com.", "detail": "NXDOMAIN from ns1.example.com" } }
```
:::

`ready` says whether the record is visible everywhere the plugin checks,
`effective_fqdn` the name it queried and `detail` why it is not ready yet. A
plugin implements this when it can check faster or more reliably than public
DNS, for example through the provider's API. Without it, Nginx UI queries DNS
itself.

## Certificate Options

`options` carries what the challenge form stored with the certificate. It is
opaque to Nginx UI: whatever a plugin's
[challenge form component](../slots.md#certificate-challenge-form) writes
into `challenge_config` comes back unchanged. The official DNS-01 plugin
uses:

| Key | Meaning |
| --- | --- |
| `credential_id` | The id of the chosen credential. |
| `disable_cname` | Do not follow a CNAME of the challenge record. |
| `disable_authoritative_ns_propagation` | Skip the check against the authoritative name servers. |
| `disable_recursive_ns_propagation` | Skip the check against recursive name servers. |

`options` may be empty, for example for a certificate issued before a plugin
had a form. Use sensible defaults then rather than failing.

## Credential Form

`form` describes every value a provider takes. Nginx UI builds the credential
form from it and from nothing else, and never stores or sends a key the form
does not name. A provider without values declares `"fields": []`.

```json [plugin.json]
"form": {
  "fields": [
    { "key": "MYDNS_API_TOKEN", "label": "API token", "help": "Needs the DNS edit permission.", "group": "credential", "secret": true },
    { "key": "MYDNS_API_EMAIL", "label": "Account email", "group": "credential" },
    { "key": "MYDNS_API_KEY", "label": "API key", "group": "credential", "secret": true },
    { "key": "MYDNS_TTL", "label": "TXT record TTL", "group": "setting", "default": "120", "unit": "seconds" }
  ],
  "methods": [
    { "name": "API token", "recommended": true, "fields": ["MYDNS_API_TOKEN"] },
    { "name": "Global API key", "fields": ["MYDNS_API_EMAIL", "MYDNS_API_KEY"] },
    { "name": "Instance role", "fields": [], "values": { "MYDNS_AUTH_MODE": "instance" } }
  ]
}
```

### Fields

| Field | Required | Meaning |
| --- | --- | --- |
| `key` | yes | Key of the value in `config`. Unique in the form. |
| `label` | yes | Short label. |
| `help` | no | One sentence shown under the input. |
| `group` | yes | `credential` for a sign in value, `setting` for tuning such as a TTL, a base URL or a timeout. |
| `optional` | no | The provider works without the value. |
| `secret` | no | A password, token or key, shown as a password input. |
| `default` | no | The value used when the field is empty. Shown as a placeholder, never stored for the person. |
| `unit` | no | `seconds` for a number of seconds. |
| `link` | no | Documentation link for the field. |

Nginx UI shows `setting` fields apart from the credentials, and stores the two
groups separately, treating credentials as secret. The plugin receives both
merged into `config`. Leave out properties that are empty or `false` to keep
the manifest small.

### Sign In Methods

When a provider offers several ways to sign in, `methods` lists them, with at
least two entries:

| Field | Required | Meaning |
| --- | --- | --- |
| `name` | yes | Name of the method. Unique. |
| `recommended` | no | Preselect this method. At most one. |
| `fields` | yes | Keys of the `credential` fields this method uses. Empty for a method without input. |
| `values` | no | Fixed values that select this method in the plugin, such as an authentication mode. |

- A credential field no method lists is shared and shown with every method.
  Nginx UI shows only the fields of the chosen method plus the shared ones,
  and on save clears the credential values the method does not use.
- `values` are stored with the credential and sent in `config`. A key of
  `values` is usually not a field. It may be a credential field that some
  methods list and others fix: then methods listing it show it, and methods
  setting it fix its value. No method both lists and sets the same key.
- No two methods may have the same fields and the same values.

When a saved credential is edited, Nginx UI preselects the method whose
`values` match and whose fields hold values, otherwise the recommended one,
otherwise the first.

### Translations

The provider `name`, every `label` and `help` and every method `name` are
English source strings. Nginx UI translates them with the translations the
plugin's browser bundle registers with `registerTranslations`, and shows the
English text when a translation is missing.

A form that breaks any rule of this section fails the
[`dns01-form`](../rules.md#dns01-form) check, and Nginx UI may refuse to
install the plugin.
