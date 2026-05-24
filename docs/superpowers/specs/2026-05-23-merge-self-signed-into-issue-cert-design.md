# Merge Self-signed Certificate into Issue Certificate Dialog

Date: 2026-05-23
Author: brainstorming session with Jacky
Status: approved

## Background

The certificate list page (`app/src/views/certificate/CertificateList/Certificate.vue`)
currently exposes three actions in the card header:

1. **Import** — navigates to `/certificates/import`
2. **Self-signed Certificate** — opens `SelfSignedCertForm.vue` modal
3. **Issue certificate** — opens `DNSIssueCertificate.vue` modal (ACME wildcard / custom domain)

The Self-signed and Issue flows produce certificates by different means but land in
the same list, and the two-button arrangement is redundant from the user's
perspective. We will consolidate them so that "Issue certificate" is the single
entry point for creating a new certificate, with ACME and self-signed exposed as
alternative *certificate types* inside that dialog.

## Goal

* Header collapses from three actions to two: **Import** and **Issue certificate**.
* The Issue Certificate dialog gains a `Self-signed` option in its existing
  Certificate Type dropdown.
* Selecting `Self-signed` swaps the form body to the self-signed field set and
  routes submission through the existing self-signed generation API.
* No backend changes.

## Non-goals

* No changes to the site-editor self-signed shortcut
  (`app/src/views/site/site_edit/components/Cert/SelfSignedCert.vue`) — it
  continues to use `SelfSignedCertForm.vue` directly with default domains.
* No changes to the editor view of an existing self-signed certificate
  (`SelfSignedCertManagement.vue`).
* No rename of `DNSIssueCertificate.vue` (would only churn i18n `.pot` refs).
* No backend / API / Go test changes.

## Affected files

| File | Change |
| --- | --- |
| `app/src/views/certificate/components/DNSIssueCertificate.vue` | Extend `certType` with `'self_signed'`; render `SelfSignedCertFields` and call self-signed API when selected. |
| `app/src/views/certificate/CertificateList/Certificate.vue` | Remove the standalone Self-signed button and related imports / refs / handlers. |
| `app/src/views/certificate/components/SelfSignedCertForm.vue` | **Untouched.** Still used by the site editor. |
| `app/src/views/certificate/components/SelfSignedCertFields.vue` | **Untouched.** Reused as the field set inside the merged dialog. |

## Design

### DNSIssueCertificate.vue

State additions:

* `certType: Ref<'wildcard' | 'custom' | 'self_signed'>` — extend the existing
  union with `'self_signed'`.
* `selfSignedPayload: Ref<SelfSignedCertPayload>` — mirror the shape used by
  `SelfSignedCertForm.emptyForm()`:
  ```ts
  {
    name: '',
    domains: [],
    ip_addresses: [],
    key_type: PrivateKeyTypeEnum.P256,
    validity_days: 365,
    sync_node_ids: [],
  }
  ```
* `selfSignedLoading: Ref<boolean>` — disables the Generate button while the
  POST is in flight.

`open()` resets `selfSignedPayload` alongside the existing resets.

Template:

* The Certificate Type select gets a third `<ASelectOption value="self_signed">`
  labelled `Self-signed Certificate`.
* `v-if="certType === 'self_signed'"` branch renders
  `<SelfSignedCertFields v-model="selfSignedPayload" />` only.
  * Hides: wildcard domain input, custom-domains list, the
    `AutoCertForm` block, the `ObtainCertLive` step.
* Footer button:
  * `certType === 'self_signed'` → button label `Generate`, calls a new
    `submitSelfSigned()`, shows `selfSignedLoading` spinner.
  * Other modes → unchanged `Next` button calling `issueCert()`.

`submitSelfSigned()`:

```ts
async function submitSelfSigned() {
  const { domains, ip_addresses } = selfSignedPayload.value
  if (domains.length === 0 && ip_addresses.length === 0) {
    message.error($gettext('Please enter at least one domain or IP address'))
    return
  }
  selfSignedLoading.value = true
  try {
    await cert.generate_self_signed(selfSignedPayload.value)
    message.success($gettext('Self-signed certificate generated'))
    visible.value = false
    emit('issued')
  }
  // eslint-disable-next-line ts/no-explicit-any
  catch (e: any) {
    message.error(e.message ?? $gettext('Failed to generate self-signed certificate'))
  }
  finally {
    selfSignedLoading.value = false
  }
}
```

(`message` already exists via `App.useApp()` in the component; the wording
matches the current `SelfSignedCertForm` exactly so translations can be reused.)

Behavioral notes:

* Switching `certType` does not wipe other branches' state — user can switch
  back and forth without losing input.
* `step` only advances on the ACME path; the self-signed path stays at the
  form view and closes on success.

### Certificate.vue

Remove:

* The `SafetyOutlined` import.
* The `SelfSignedCertForm` import.
* `import type { Cert } from '@/api/cert'` (only used by `onSelfSignedCreated`).
* The `refSelfSigned` ref.
* The `onSelfSignedCreated` function.
* The `<AButton>` block rendering the Self-signed Certificate action.
* The `<SelfSignedCertForm ref="refSelfSigned" @created="onSelfSignedCreated" />`
  block at the bottom of the template.

Result: header has only Import + Issue certificate; `<WildcardCertificate>`
(name preserved, semantically now "Issue Certificate") remains the single
modal.

### Behavioural change

After a self-signed certificate is generated through the merged dialog:

* **Before:** `router.push('/certificates/:id')` from `onSelfSignedCreated`.
* **After:** dialog closes + table refreshes via the existing `issued` emit,
  matching the ACME completion behaviour.

Rationale: the merged dialog has one exit contract. Users can click the new row
to open the editor; this is consistent with ACME flow and avoids a context jump
that ACME users don't experience.

## Testing

* Frontend gates: `pnpm lint`, `pnpm lint:fix`, `pnpm typecheck` must all pass.
* Manual verification:
  1. Certificate list header shows only Import + Issue certificate.
  2. Issue Certificate dialog defaults to Wildcard; ACME flow still works end-to-end.
  3. Switching to Custom Domains still works (no regression).
  4. Switching to Self-signed shows the self-signed field set; Generate creates
     a row and closes the dialog with the table refreshed.
  5. Validation: clicking Generate with no domain and no IP shows the existing
     "Please enter at least one domain or IP address" error.
  6. Site editor → SelfSignedCert.vue still opens its own modal and works
     unchanged (regression check on the untouched code path).
* No backend changes → existing Go tests remain authoritative; no new Go tests
  required for this UI refactor.

## i18n

* Existing key `Self-signed Certificate` is already in the catalog (current
  button label) — reuse for the dropdown option label.
* `Generate`, `Please enter at least one domain or IP address`,
  `Self-signed certificate generated`,
  `Failed to generate self-signed certificate` already exist in
  `SelfSignedCertForm.vue`.
* No new strings required.

## Rollout

* Single PR against `dev`.
* Reviewer touchpoints: certificate list header, Issue Certificate dialog,
  regression check on site editor self-signed path.
