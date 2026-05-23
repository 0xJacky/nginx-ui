# Self-signed Certificate UX Enhancements

Date: 2026-05-23
Author: brainstorming session with Jacky
Status: approved
Builds on: [2026-05-23-merge-self-signed-into-issue-cert-design.md](./2026-05-23-merge-self-signed-into-issue-cert-design.md)

## Background

After the initial merge of the self-signed creation flow into the Issue
Certificate dialog, three follow-up improvements emerged from manual testing:

1. **Inconsistent editors.** The Custom Domains branch uses a multi-row
   `AInput` list with explicit Add/Remove buttons, while the self-signed
   Domains and IP Addresses use a chip-style tags select. Same dialog, two
   editing models.
2. **Missing auto-renewal context.** Users creating a self-signed cert have
   no in-form indication that Nginx UI will renew it automatically; the
   policy depends on a global setting and the per-cert validity period.
3. **Empty Name slips through.** The backend silently accepts an empty
   `Name`, which leaves the certificate list with blank rows and forces the
   file-system slug to fall back to the first domain or IP. For self-signed
   certificates we want Name to be required.

## Goals

* Single, reusable list-input component used by Custom Domains and self-signed
  Domains/IPs.
* Inline policy hint inside the self-signed form so users understand renewal
  expectations at creation time.
* Self-signed certificates require a non-empty Name (enforced on both client
  and server).

## Non-goals

* No client-side IP format validation; the backend already enforces it via
  `binding:"omitempty,dive,ip"` and surfaces the error in the toast.
* No DB migration of existing self-signed rows with empty names. They stay
  as-is until the user opens and re-saves them (at which point the new
  required validation kicks in).
* No rename of `DNSIssueCertificate.vue`; same scope discipline as before.
* No change to ACME (Wildcard / Custom Domains) submit semantics.

## Affected files

New:

| File | Responsibility |
| --- | --- |
| `app/src/components/StringListInput/StringListInput.vue` | Reusable multi-row string-array input with Add / Remove. |
| `app/src/components/StringListInput/index.ts` | Re-export. |

Modified:

| File | Change |
| --- | --- |
| `app/src/views/certificate/components/DNSIssueCertificate.vue` | Custom Domains uses `StringListInput`; submitSelfSigned trims/filters and validates Name; seeds payload with `domains: ['']`, `ip_addresses: ['']`. |
| `app/src/views/certificate/components/SelfSignedCertFields.vue` | Domains/IPs use `StringListInput`; adds renewal-policy `AAlert` (with `hideRenewalNote?: boolean` prop to suppress); Name becomes required field. |
| `app/src/views/certificate/components/SelfSignedCertForm.vue` | `emptyForm()` seeds empty-row arrays; `submit()` trims/filters and validates Name. |
| `app/src/views/certificate/components/SelfSignedCertManagement.vue` | Passes `hide-renewal-note` to `SelfSignedCertFields` to avoid double alert. |
| `app/src/views/certificate/CertificateEditor.vue` | `save()` trims/filters arrays and validates Name before `modify_self_signed`. |
| `app/src/api/cert.ts` | `toSelfSignedPayload()` seeds empty-row arrays. |
| `api/certificate/self_signed.go` | `SelfSignedCertRequest.Name` gets `binding:"required"`. |
| `api/certificate/self_signed_test.go` | Add test that POST `/self_signed_cert` with empty Name returns 400. |

Untouched:

| File | Reason |
| --- | --- |
| `app/src/views/site/site_edit/components/Cert/SelfSignedCert.vue` | Calls `SelfSignedCertForm` with `defaultDomains`; downstream changes propagate automatically. |
| `internal/cert/self_signed.go` and other backend helpers | Validation lives at the request boundary. |

## Component: `StringListInput`

API:

```ts
interface Props {
  placeholder?: string
  addButtonText?: string  // default: $gettext('Add Item')
  // No validator prop in v1 — YAGNI.
}
```

`v-model: string[]` (required). The model array may legitimately contain a
single empty string while the user is composing the first value. Consumers
are responsible for filtering empties on submit.

Behaviour:

* Renders one `AInput` per array entry.
* `Remove` link (red) shown when array length > 1; clicking it splices that
  index out.
* Block `AButton` at the bottom labelled by `addButtonText` (default `Add Item`),
  pushes `''` onto the array.
* No internal validation; uses `:placeholder` for hint text.

This is literally the pattern from the existing `Custom Domains` branch in
`DNSIssueCertificate.vue` — moved into its own file and parameterized.

## SelfSignedCertFields changes

* Replace the Domains `<ASelect mode="tags">` with `<StringListInput v-model="data.domains" :placeholder="$gettext('Enter domain name')" :add-button-text="$gettext('Add Domain')" />`.
* Replace the IP Addresses `<ASelect mode="tags">` with `<StringListInput v-model="data.ip_addresses" :placeholder="$gettext('Enter IP address')" :add-button-text="$gettext('Add IP Address')" />`.
* Name field:
  * Update wrapping `<AFormItem>` to set `required` for the asterisk.
  * Change placeholder from `Optional` to `Enter certificate name`.
* Add new prop `hideRenewalNote?: boolean` (defaults `false`).
* When `!hideRenewalNote`, render at the top of the form:
  ```vue
  <AAlert
    class="mb-4"
    type="info"
    show-icon
    :message="$gettext('Nginx UI will automatically renew this certificate as it approaches expiration, based on the global certificate renewal interval and this certificate\'s validity period.')"
  />
  ```

## Payload seeding & filtering

To present an empty editable row, payload factories seed with `['']`:

* `emptySelfSignedPayload()` in `DNSIssueCertificate.vue` → `domains: ['']`, `ip_addresses: ['']`
* `emptyForm()` in `SelfSignedCertForm.vue` → `domains: defaultDomains?.length ? [...defaultDomains] : ['']`, `ip_addresses: ['']`
* `toSelfSignedPayload(c)` in `cert.ts` → `domains: c.domains?.length ? [...c.domains] : ['']`, `ip_addresses: c.self_signed_config?.ip_addresses?.length ? [...c.self_signed_config.ip_addresses] : ['']`

Each submit/save path trims and filters before validation:

```ts
const name = (payload.name ?? '').trim()
const domains = payload.domains.map(d => d.trim()).filter(Boolean)
const ip_addresses = payload.ip_addresses.map(s => s.trim()).filter(Boolean)

if (!name) {
  message.error($gettext('Please enter a name for the certificate'))
  return
}
if (domains.length === 0 && ip_addresses.length === 0) {
  message.error($gettext('Please enter at least one domain or IP address'))
  return
}

await cert.generate_self_signed({ ...payload, name, domains, ip_addresses })
```

The same shape is used in:
* `DNSIssueCertificate.submitSelfSigned()`
* `SelfSignedCertForm.submit()`
* `CertificateEditor.save()` (in the `isSelfSigned` branch)

## Backend: required Name

Update `SelfSignedCertRequest` in `api/certificate/self_signed.go`:

```go
type SelfSignedCertRequest struct {
    Name         string   `json:"name" binding:"required"`
    Domains      []string `json:"domains" binding:"omitempty"`
    IPAddresses  []string `json:"ip_addresses" binding:"omitempty,dive,ip"`
    KeyType      string   `json:"key_type" binding:"omitempty,auto_cert_key_type"`
    ValidityDays int      `json:"validity_days" binding:"omitempty,min=1,max=3650"`
    SyncNodeIds  []uint64 `json:"sync_node_ids" binding:"omitempty"`
}
```

This applies to both `GenerateSelfSignedCert` and `ModifySelfSignedCert`,
since both bind the same struct via `cosy.BindAndValid`.

The CommonName fallback in `selfSignedSlug` (line ~69) stays as defensive code
even though it's now unreachable for fresh requests — cheap, and protects
direct callers of `selfSignedSlug` outside the handler.

## Testing

### Frontend

* `pnpm lint` / `pnpm lint:fix` / `pnpm typecheck` from `app/` — must pass.
* No new component tests added (project does not carry component tests for
  this view; manual smoke covers behaviour).

### Backend

* Existing self-signed Go tests must remain green: `go test ./api/certificate/... ./internal/cert/...`.
* Add `TestGenerateSelfSignedCertRejectsEmptyName` in
  `api/certificate/self_signed_test.go`:
  * POST `/self_signed_cert` with `{"domains":["a.test"],"validity_days":30}`
    (no `name`) → expect HTTP 4xx and an error payload referencing the
    `name` field. Use the same `setupSelfSignedAPITest` helper as the rollback
    test.
* No DB migration tests required — existing rows are not touched.

### Manual smoke

1. Open Issue Certificate → Self-signed:
   * Renewal alert visible at the top.
   * Name field shows red asterisk; placeholder `Enter certificate name`.
   * Domains and IPs each render as multi-row inputs with `Add Domain` / `Add IP Address`.
2. Click `Generate` with everything empty → `Please enter a name for the certificate`.
3. Fill Name only → `Please enter at least one domain or IP address`.
4. Fill Name + one domain → cert is created; list refreshes; row has the name.
5. Edit an existing self-signed cert that has a Name → save still works.
6. Edit an existing self-signed cert with empty Name (if any exist) → save is
   blocked until Name is filled.
7. Site editor → Self-signed shortcut: alert visible (same field component);
   Name required.
8. Editor for an existing self-signed cert → `SelfSignedCertManagement`'s own
   "managed by Nginx UI and renewed automatically" alert remains; the new
   policy alert is NOT shown (suppressed by `hide-renewal-note`).
9. Custom Domains branch in Issue Certificate dialog still renders identically
   to before (visual regression check).

## i18n

New strings (English source):
* `Add Item`
* `Add Domain` (existing)
* `Add IP Address`
* `Enter domain name` (existing)
* `Enter IP address`
* `Enter certificate name`
* `Please enter a name for the certificate`
* `Nginx UI will automatically renew this certificate as it approaches expiration, based on the global certificate renewal interval and this certificate's validity period.`

The strings already present in the original `SelfSignedCertForm` continue to
be reused. `messages.pot` regeneration is a separate operational follow-up
(noted in the prior PR's review) and is out of scope for this PR.

## Rollout

* Single PR on top of the two already-landed commits on `feature/self-signed-certificate`.
* Reviewer touchpoints: new component, three consumer wirings, backend
  binding change, Go test addition.
