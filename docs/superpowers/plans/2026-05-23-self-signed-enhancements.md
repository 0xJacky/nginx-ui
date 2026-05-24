# Self-signed Certificate UX Enhancements Implementation Plan

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** (1) Extract the Custom Domains row-list editor into a shared `StringListInput` component and reuse it for the self-signed Domains / IP Addresses fields. (2) Show a renewal-policy hint in the self-signed form. (3) Require a non-empty Name for self-signed certificates on both client and server.

**Architecture:** New `StringListInput` lives in `app/src/components/`. `SelfSignedCertFields` gains a `hideRenewalNote` prop and required Name. Payload factories in three locations seed an empty editable row; three submit/save paths trim and filter before calling the API and now reject empty Name. Backend gains `binding:"required"` on `SelfSignedCertRequest.Name` and one new Go test.

**Tech Stack:** Vue 3 `<script setup lang="ts">`, Ant Design Vue, UnoCSS, project's `$gettext` helper. Go / Gin / Cosy for the backend; existing `gin.New` + `httptest` test pattern.

**Spec:** [`docs/superpowers/specs/2026-05-23-self-signed-enhancements-design.md`](../specs/2026-05-23-self-signed-enhancements-design.md)

**Prior context:** Builds on commits `605c6fed1` and `19776d442` on the `feature/self-signed-certificate` branch.

---

## Project conventions

- Frontend: pnpm only. Lint/typecheck gates run from `app/`: `pnpm lint`, `pnpm lint:fix`, `pnpm typecheck`. The `perfectionist` rule may reorder imports during `lint:fix` — accept it.
- Vue auto-imports: `ref`, `computed`, `watch`, `App.useApp`, `$gettext`, `useTemplateRef`, `storeToRefs`. Match surrounding style.
- All comments / i18n strings in English.
- Backend: `gofmt`/`goimports` clean, `go test ./... -race -cover` for full sweep but only the touched packages need to pass.
- Commits: imperative subject, sign-off:
  `Co-Authored-By: Claude Opus 4.7 (1M context) <noreply@anthropic.com>`
- Branch: `feature/self-signed-certificate`. Commit directly; do not branch.

---

## File map

New:

- `app/src/components/StringListInput/StringListInput.vue`
- `app/src/components/StringListInput/index.ts`

Modified:

- `app/src/views/certificate/components/SelfSignedCertFields.vue`
- `app/src/views/certificate/components/SelfSignedCertForm.vue`
- `app/src/views/certificate/components/SelfSignedCertManagement.vue`
- `app/src/views/certificate/components/DNSIssueCertificate.vue`
- `app/src/views/certificate/CertificateEditor.vue`
- `app/src/api/cert.ts`
- `api/certificate/self_signed.go`
- `api/certificate/self_signed_test.go`

---

### Task 1: Create the StringListInput component

**Files:**
- Create: `app/src/components/StringListInput/StringListInput.vue`
- Create: `app/src/components/StringListInput/index.ts`

- [ ] **Step 1: Create the component file**

Write `app/src/components/StringListInput/StringListInput.vue` exactly:

```vue
<script setup lang="ts">
defineProps<{
  placeholder?: string
  addButtonText?: string
}>()

const items = defineModel<string[]>({ required: true })

function addItem() {
  items.value = [...items.value, '']
}

function removeItem(index: number) {
  if (items.value.length <= 1)
    return
  const next = [...items.value]
  next.splice(index, 1)
  items.value = next
}

function updateItem(index: number, value: string) {
  const next = [...items.value]
  next[index] = value
  items.value = next
}
</script>

<template>
  <div class="space-y-2">
    <div
      v-for="(item, index) in items"
      :key="index"
      class="flex items-center gap-2"
    >
      <AInput
        :value="item"
        :placeholder="placeholder"
        class="flex-1"
        @update:value="(value: string) => updateItem(index, value)"
      />
      <AButton
        v-if="items.length > 1"
        type="link"
        danger
        @click="removeItem(index)"
      >
        {{ $gettext('Remove') }}
      </AButton>
    </div>
    <AButton
      block
      @click="addItem"
    >
      {{ addButtonText ?? $gettext('Add Item') }}
    </AButton>
  </div>
</template>
```

Notes:
- Uses `defineModel<string[]>({ required: true })` — Vue 3.4+ idiom, already used elsewhere in this project.
- `updateItem` swaps via spread to keep the reactive identity stable (mirrors how the original Custom Domains template mutates `customDomains[index]` via two-way binding; we go through an explicit setter so the model emits cleanly).

- [ ] **Step 2: Create the re-export**

Write `app/src/components/StringListInput/index.ts`:

```ts
import StringListInput from './StringListInput.vue'

export default StringListInput
```

- [ ] **Step 3: Lint and typecheck**

```bash
cd app && pnpm lint && pnpm typecheck
```

Expected: both exit 0. If lint flags ordering, run `pnpm lint:fix` then re-check.

- [ ] **Step 4: Commit**

```bash
git add app/src/components/StringListInput/
git commit -m "$(cat <<'EOF'
feat(ui): add StringListInput component

Reusable multi-row text input with Add/Remove buttons. Used in the
upcoming refactor of Custom Domains and self-signed Domains / IP
Addresses editors so all three share a single editor pattern.

Co-Authored-By: Claude Opus 4.7 (1M context) <noreply@anthropic.com>
EOF
)"
```

---

### Task 2: Make Name required on the backend and add a covering Go test

Doing the backend first means the frontend changes in later tasks are validated against the new server contract end-to-end.

**Files:**
- Modify: `api/certificate/self_signed.go`
- Modify: `api/certificate/self_signed_test.go`

- [ ] **Step 1: Add `binding:"required"` to Name**

In `api/certificate/self_signed.go`, change the struct definition:

```go
type SelfSignedCertRequest struct {
	Name         string   `json:"name"`
```

to:

```go
type SelfSignedCertRequest struct {
	Name         string   `json:"name" binding:"required"`
```

(Only the `Name` field's tag changes; the other fields stay as-is.)

- [ ] **Step 2: Add the failing test first**

Append a new test to `api/certificate/self_signed_test.go`. Use the same pattern as the existing rollback test (which sets up `setupSelfSignedAPITest`, creates a gin router, marshals a `SelfSignedCertRequest`, POSTs via `httptest`).

```go
func TestGenerateSelfSignedCertRejectsEmptyName(t *testing.T) {
	setupSelfSignedAPITest(t)

	router := gin.New()
	router.POST("/self_signed_cert", GenerateSelfSignedCert)

	body, err := json.Marshal(SelfSignedCertRequest{
		Domains:      []string{"named.example"},
		KeyType:      string(certcrypto.EC256),
		ValidityDays: 30,
	})
	if err != nil {
		t.Fatalf("marshal request: %v", err)
	}

	req := httptest.NewRequest(http.MethodPost, "/self_signed_cert", bytes.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	rec := httptest.NewRecorder()
	router.ServeHTTP(rec, req)

	if rec.Code < 400 || rec.Code >= 500 {
		t.Fatalf("status = %d, want a 4xx for missing name", rec.Code)
	}
	if !strings.Contains(strings.ToLower(rec.Body.String()), "name") {
		t.Fatalf("response body %q did not mention the missing name field", rec.Body.String())
	}
}
```

Check existing imports in the test file. If `bytes`, `http`, `httptest`, `strings` are not already imported, add them. The rollback test already imports `bytes`, `encoding/json`, `net/http`, `net/http/httptest`, and `gin`, so most should be present — only `strings` may need adding.

- [ ] **Step 3: Run the new test to confirm it passes**

```bash
go test ./api/certificate/ -run TestGenerateSelfSignedCertRejectsEmptyName -race
```

Expected: PASS.

- [ ] **Step 4: Run the full self-signed test set to confirm no regression**

```bash
go test ./api/certificate/ ./internal/cert/ -race
```

Expected: all PASS. The existing `TestGenerateSelfSignedCertRollsBackDBOnFileWriteFailure` already sends `Name: "rollback-test"` so it's unaffected; `buildSelfSignedOptions` unit tests don't go through binding validation.

- [ ] **Step 5: Lint formatting**

```bash
gofmt -w api/certificate/self_signed.go api/certificate/self_signed_test.go
goimports -w api/certificate/self_signed.go api/certificate/self_signed_test.go 2>/dev/null || true
```

(If `goimports` isn't installed locally that's fine; `gofmt` is the hard gate.)

- [ ] **Step 6: Commit**

```bash
git add api/certificate/self_signed.go api/certificate/self_signed_test.go
git commit -m "$(cat <<'EOF'
feat(cert): require Name when generating self-signed certificates

Adds binding:"required" to SelfSignedCertRequest.Name so an empty name
is rejected at the request boundary, and covers the contract with a
new API-level test.

Co-Authored-By: Claude Opus 4.7 (1M context) <noreply@anthropic.com>
EOF
)"
```

---

### Task 3: Refactor SelfSignedCertFields to use StringListInput, add renewal hint, mark Name required

**Files:**
- Modify: `app/src/views/certificate/components/SelfSignedCertFields.vue`

- [ ] **Step 1: Replace the component file**

Replace the entire contents of `app/src/views/certificate/components/SelfSignedCertFields.vue` with:

```vue
<script setup lang="ts">
import type { SelfSignedCertPayload } from '@/api/cert'
import NodeSelector from '@/components/NodeSelector'
import StringListInput from '@/components/StringListInput'
import { PrivateKeyTypeList } from '@/constants'

const props = defineProps<{
  isKeyTypeReadonly?: boolean
  hideRenewalNote?: boolean
}>()

const data = defineModel<SelfSignedCertPayload>({ required: true })
</script>

<template>
  <AForm layout="vertical">
    <AAlert
      v-if="!props.hideRenewalNote"
      class="mb-4"
      type="info"
      show-icon
      :message="$gettext('Nginx UI will automatically renew this certificate as it approaches expiration, based on the global certificate renewal interval and this certificate\'s validity period.')"
    />
    <AFormItem
      :label="$gettext('Name')"
      required
    >
      <AInput
        v-model:value="data.name"
        :placeholder="$gettext('Enter certificate name')"
      />
    </AFormItem>
    <AFormItem :label="$gettext('Domains')">
      <StringListInput
        v-model="data.domains"
        :placeholder="$gettext('Enter domain name')"
        :add-button-text="$gettext('Add Domain')"
      />
    </AFormItem>
    <AFormItem :label="$gettext('IP Addresses')">
      <StringListInput
        v-model="data.ip_addresses"
        :placeholder="$gettext('Enter IP address')"
        :add-button-text="$gettext('Add IP Address')"
      />
    </AFormItem>
    <AFormItem :label="$gettext('Key Type')">
      <ASelect
        v-model:value="data.key_type"
        :disabled="props.isKeyTypeReadonly"
      >
        <ASelectOption
          v-for="t in PrivateKeyTypeList"
          :key="t.key"
          :value="t.key"
        >
          {{ t.name }}
        </ASelectOption>
      </ASelect>
    </AFormItem>
    <AFormItem :label="$gettext('Valid For (days)')">
      <AInputNumber
        v-model:value="data.validity_days"
        :min="1"
        :max="3650"
        class="w-full"
      />
      <template #help>
        {{ $gettext('Some browsers reject TLS certificates valid for more than 398 days.') }}
      </template>
    </AFormItem>
    <AFormItem :label="$gettext('Sync to')">
      <NodeSelector
        v-model:target="data.sync_node_ids"
        hidden-local
      />
    </AFormItem>
  </AForm>
</template>
```

Substantive changes vs. the original file:
- Added `StringListInput` import.
- Added `hideRenewalNote?: boolean` prop alongside `isKeyTypeReadonly`.
- Removed the previous Name placeholder text `Optional` and added the `required` attribute on the `<AFormItem>` for the asterisk.
- Replaced the Domains and IP Addresses `<ASelect mode="tags">` blocks with `StringListInput`.
- Added the renewal `AAlert` immediately under `<AForm>`, gated by `!hideRenewalNote`.

- [ ] **Step 2: Lint and typecheck**

```bash
cd app && pnpm lint && pnpm typecheck
```

Expected: both 0. `lint:fix` if needed.

- [ ] **Step 3: Commit**

```bash
git add app/src/views/certificate/components/SelfSignedCertFields.vue
git commit -m "$(cat <<'EOF'
feat(cert): unify self-signed editor and surface renewal hint

Switch Domains and IP Addresses to the shared StringListInput so all
self-signed field editors match the Custom Domains pattern. Add an
auto-renewal hint (suppressible via hideRenewalNote) and mark Name as
required to match the new backend contract.

Co-Authored-By: Claude Opus 4.7 (1M context) <noreply@anthropic.com>
EOF
)"
```

---

### Task 4: Update SelfSignedCertManagement to suppress the duplicate alert

**Files:**
- Modify: `app/src/views/certificate/components/SelfSignedCertManagement.vue`

This view already shows its own `type="success"` "managed by Nginx UI and renewed automatically" alert. We don't want both.

- [ ] **Step 1: Add the `hide-renewal-note` attribute**

In `app/src/views/certificate/components/SelfSignedCertManagement.vue`, find the `<SelfSignedCertFields>` element. Replace:

```vue
    <SelfSignedCertFields
      v-model="data"
      is-key-type-readonly
    />
```

with:

```vue
    <SelfSignedCertFields
      v-model="data"
      is-key-type-readonly
      hide-renewal-note
    />
```

- [ ] **Step 2: Lint and typecheck**

```bash
cd app && pnpm lint && pnpm typecheck
```

Expected: both 0.

- [ ] **Step 3: Commit**

```bash
git add app/src/views/certificate/components/SelfSignedCertManagement.vue
git commit -m "$(cat <<'EOF'
chore(cert): suppress duplicate renewal alert in cert editor

SelfSignedCertManagement already has its own renewal-status alert;
pass hide-renewal-note to SelfSignedCertFields to avoid showing two
adjacent alerts saying the same thing.

Co-Authored-By: Claude Opus 4.7 (1M context) <noreply@anthropic.com>
EOF
)"
```

---

### Task 5: Seed and filter payloads + validate Name in the three submit/save paths

**Files:**
- Modify: `app/src/api/cert.ts`
- Modify: `app/src/views/certificate/components/SelfSignedCertForm.vue`
- Modify: `app/src/views/certificate/components/DNSIssueCertificate.vue`
- Modify: `app/src/views/certificate/CertificateEditor.vue`

The new `StringListInput` keeps an empty placeholder row in the model array. Each writing path needs to (a) seed `['']` for empty arrays so the editor renders an empty row, and (b) trim + filter + validate Name before sending.

- [ ] **Step 1: Update `toSelfSignedPayload` in `app/src/api/cert.ts`**

Find:

```ts
export function toSelfSignedPayload(c: Cert): SelfSignedCertPayload {
  return {
    name: c.name ?? '',
    domains: [...(c.domains ?? [])],
    ip_addresses: [...(c.self_signed_config?.ip_addresses ?? [])],
    key_type: c.key_type || PrivateKeyTypeEnum.P256,
    validity_days: c.self_signed_config?.validity_days || 365,
    sync_node_ids: [...(c.sync_node_ids ?? [])],
  }
}
```

Replace with:

```ts
export function toSelfSignedPayload(c: Cert): SelfSignedCertPayload {
  const domains = c.domains?.length ? [...c.domains] : ['']
  const ipAddresses = c.self_signed_config?.ip_addresses?.length
    ? [...c.self_signed_config.ip_addresses]
    : ['']
  return {
    name: c.name ?? '',
    domains,
    ip_addresses: ipAddresses,
    key_type: c.key_type || PrivateKeyTypeEnum.P256,
    validity_days: c.self_signed_config?.validity_days || 365,
    sync_node_ids: [...(c.sync_node_ids ?? [])],
  }
}
```

- [ ] **Step 2: Update `SelfSignedCertForm.vue`**

Find `emptyForm()`:

```ts
function emptyForm(): SelfSignedCertPayload {
  return {
    name: '',
    domains: [...(props.defaultDomains ?? [])],
    ip_addresses: [],
    key_type: PrivateKeyTypeEnum.P256,
    validity_days: 365,
    sync_node_ids: [],
  }
}
```

Replace with:

```ts
function emptyForm(): SelfSignedCertPayload {
  const defaultDomains = props.defaultDomains ?? []
  return {
    name: '',
    domains: defaultDomains.length ? [...defaultDomains] : [''],
    ip_addresses: [''],
    key_type: PrivateKeyTypeEnum.P256,
    validity_days: 365,
    sync_node_ids: [],
  }
}
```

Find `submit()`:

```ts
async function submit() {
  if (form.value.domains.length === 0 && form.value.ip_addresses.length === 0) {
    message.error($gettext('Please enter at least one domain or IP address'))
    return
  }

  loading.value = true
  try {
    const created = await cert.generate_self_signed(form.value)
    message.success($gettext('Self-signed certificate generated'))
    visible.value = false
    emit('created', created)
  }
  // eslint-disable-next-line ts/no-explicit-any
  catch (e: any) {
    message.error(e.message ?? $gettext('Failed to generate self-signed certificate'))
  }
  finally {
    loading.value = false
  }
}
```

Replace with:

```ts
async function submit() {
  const name = (form.value.name ?? '').trim()
  const domains = form.value.domains.map(d => d.trim()).filter(Boolean)
  const ip_addresses = form.value.ip_addresses.map(s => s.trim()).filter(Boolean)

  if (!name) {
    message.error($gettext('Please enter a name for the certificate'))
    return
  }
  if (domains.length === 0 && ip_addresses.length === 0) {
    message.error($gettext('Please enter at least one domain or IP address'))
    return
  }

  loading.value = true
  try {
    const created = await cert.generate_self_signed({
      ...form.value,
      name,
      domains,
      ip_addresses,
    })
    message.success($gettext('Self-signed certificate generated'))
    visible.value = false
    emit('created', created)
  }
  // eslint-disable-next-line ts/no-explicit-any
  catch (e: any) {
    message.error(e.message ?? $gettext('Failed to generate self-signed certificate'))
  }
  finally {
    loading.value = false
  }
}
```

- [ ] **Step 3: Update `DNSIssueCertificate.vue`**

Find `emptySelfSignedPayload()`:

```ts
function emptySelfSignedPayload(): SelfSignedCertPayload {
  return {
    name: '',
    domains: [],
    ip_addresses: [],
    key_type: PrivateKeyTypeEnum.P256,
    validity_days: 365,
    sync_node_ids: [],
  }
}
```

Replace with:

```ts
function emptySelfSignedPayload(): SelfSignedCertPayload {
  return {
    name: '',
    domains: [''],
    ip_addresses: [''],
    key_type: PrivateKeyTypeEnum.P256,
    validity_days: 365,
    sync_node_ids: [],
  }
}
```

Find `submitSelfSigned()`:

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

Replace with:

```ts
async function submitSelfSigned() {
  const name = (selfSignedPayload.value.name ?? '').trim()
  const domains = selfSignedPayload.value.domains.map(d => d.trim()).filter(Boolean)
  const ip_addresses = selfSignedPayload.value.ip_addresses.map(s => s.trim()).filter(Boolean)

  if (!name) {
    message.error($gettext('Please enter a name for the certificate'))
    return
  }
  if (domains.length === 0 && ip_addresses.length === 0) {
    message.error($gettext('Please enter at least one domain or IP address'))
    return
  }

  selfSignedLoading.value = true
  try {
    await cert.generate_self_signed({
      ...selfSignedPayload.value,
      name,
      domains,
      ip_addresses,
    })
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

- [ ] **Step 4: Update `CertificateEditor.vue`**

Find the `save()` function. The current `isSelfSigned` branch (around lines 58-66) is:

```ts
async function save() {
  try {
    let savedId = data.value.id
    if (isSelfSigned.value && selfSignedPayload.value && data.value.id) {
      const currentId = data.value.id
      const result = await cert.modify_self_signed(currentId, selfSignedPayload.value)
      savedId = result.id || currentId
      data.value = { ...result, id: savedId }
    }
    else {
      await certStore.save()
      savedId = data.value.id
    }
```

Modify the `isSelfSigned` branch so it trims and validates before calling the API. Replace lines from `if (isSelfSigned.value && ...)` through the closing `}` of that branch with:

```ts
    if (isSelfSigned.value && selfSignedPayload.value && data.value.id) {
      const payload = selfSignedPayload.value
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

      const currentId = data.value.id
      const result = await cert.modify_self_signed(currentId, {
        ...payload,
        name,
        domains,
        ip_addresses,
      })
      savedId = result.id || currentId
      data.value = { ...result, id: savedId }
    }
```

(Leave the `else` branch and the rest of `save()` exactly as it is.)

- [ ] **Step 5: Lint and typecheck**

```bash
cd app && pnpm lint && pnpm typecheck
```

Expected: both 0.

- [ ] **Step 6: Commit**

```bash
git add app/src/api/cert.ts \
        app/src/views/certificate/components/SelfSignedCertForm.vue \
        app/src/views/certificate/components/DNSIssueCertificate.vue \
        app/src/views/certificate/CertificateEditor.vue
git commit -m "$(cat <<'EOF'
feat(cert): seed and filter self-signed payloads, validate Name

StringListInput preserves empty placeholder rows for editing; seed
arrays with [''] in toSelfSignedPayload / emptySelfSignedPayload /
emptyForm so the editor always renders an empty row to type into.

Each submit/save path trims and filters the arrays before sending and
now rejects an empty Name client-side to match the new server contract.

Co-Authored-By: Claude Opus 4.7 (1M context) <noreply@anthropic.com>
EOF
)"
```

---

### Task 6: Switch Custom Domains in DNSIssueCertificate to StringListInput

This is a pure refactor — same UX, less duplication.

**Files:**
- Modify: `app/src/views/certificate/components/DNSIssueCertificate.vue`

- [ ] **Step 1: Add the import**

Inside the `<script setup>` block, add (next to the existing `SelfSignedCertFields` import):

```ts
import StringListInput from '@/components/StringListInput'
```

- [ ] **Step 2: Replace the inline Custom Domains list with StringListInput**

In the `<template>` block, find:

```vue
          <template v-else-if="certType === 'custom'">
            <AFormItem :label="$gettext('Custom Domains')">
              <div class="space-y-2">
                <div
                  v-for="(_, index) in customDomains"
                  :key="index"
                  class="flex items-center gap-2"
                >
                  <AInput
                    v-model:value="customDomains[index]"
                    :placeholder="$gettext('Enter domain name')"
                    class="flex-1"
                  />
                  <AButton
                    v-if="customDomains.length > 1"
                    type="link"
                    danger
                    @click="removeCustomDomain(index)"
                  >
                    {{ $gettext('Remove') }}
                  </AButton>
                </div>
                <AButton
                  block
                  @click="addCustomDomain"
                >
                  {{ $gettext('Add Domain') }}
                </AButton>
              </div>

              <AAlert
                :message="$gettext('All selected subdomains must belong to the same DNS Provider, otherwise the certificate application will fail.')"
                type="info"
                show-icon
                banner
                class="mt-3"
              />
            </AFormItem>
          </template>
```

Replace with:

```vue
          <template v-else-if="certType === 'custom'">
            <AFormItem :label="$gettext('Custom Domains')">
              <StringListInput
                v-model="customDomains"
                :placeholder="$gettext('Enter domain name')"
                :add-button-text="$gettext('Add Domain')"
              />
              <AAlert
                :message="$gettext('All selected subdomains must belong to the same DNS Provider, otherwise the certificate application will fail.')"
                type="info"
                show-icon
                banner
                class="mt-3"
              />
            </AFormItem>
          </template>
```

- [ ] **Step 3: Remove the now-unused `addCustomDomain` and `removeCustomDomain` helpers**

In the `<script setup>` block, find and delete:

```ts
function addCustomDomain() {
  customDomains.value.push('')
}

function removeCustomDomain(index: number) {
  if (customDomains.value.length > 1) {
    customDomains.value.splice(index, 1)
  }
}
```

(Keep the `customDomains` ref and the `computedDomains` / `computedMainDomain` references — they're still used by `issueCert`.)

- [ ] **Step 4: Lint and typecheck**

```bash
cd app && pnpm lint && pnpm typecheck
```

Expected: both 0. `pnpm typecheck` should catch any orphaned references to `addCustomDomain` / `removeCustomDomain`.

- [ ] **Step 5: Commit**

```bash
git add app/src/views/certificate/components/DNSIssueCertificate.vue
git commit -m "$(cat <<'EOF'
refactor(cert): use StringListInput for Custom Domains

Drop the inline multi-row template + add/remove helpers in favour of
the shared StringListInput component, matching the editor used by the
self-signed branch.

Co-Authored-By: Claude Opus 4.7 (1M context) <noreply@anthropic.com>
EOF
)"
```

---

### Task 7: Final lint/test sweep + manual verification handoff

**Files:** none modified.

- [ ] **Step 1: Final frontend gate**

```bash
cd app && pnpm lint && pnpm typecheck
```

Expected: both 0.

- [ ] **Step 2: Final backend gate**

```bash
cd .. && go test ./api/certificate/... ./internal/cert/... -race
```

Expected: all PASS, including the new `TestGenerateSelfSignedCertRejectsEmptyName`.

- [ ] **Step 3: Manual smoke checklist for the user**

(Hand off to the human — they boot the app, you don't.)

Confirm:
1. Issue Certificate → Self-signed:
   - Renewal alert visible at the top.
   - Name field has a red asterisk; placeholder reads `Enter certificate name`.
   - Domains row appears as a single empty `AInput` + Add Domain block button.
   - IP Addresses row appears the same way with Add IP Address.
2. Generate with everything empty → toast: `Please enter a name for the certificate`.
3. Fill Name only → toast: `Please enter at least one domain or IP address`.
4. Fill Name + one domain → cert created, dialog closes, table refreshes, list shows the name.
5. Open an existing self-signed cert in the editor → SelfSignedCertManagement's own renewal alert remains; no second alert from SelfSignedCertFields.
6. Edit existing cert with empty Name → save blocked until Name is filled.
7. Site editor self-signed shortcut → renewal alert visible; Name required.
8. Custom Domains branch in Issue Certificate dialog → unchanged appearance and behaviour (visual regression check).

No commit produced by this task.

---

## Wrap-up checklist

- [ ] All 6 code commits land on `feature/self-signed-certificate`.
- [ ] `pnpm lint`, `pnpm typecheck`, and the Go test sweep are all green on the final tree.
- [ ] `git status` is clean.
- [ ] Manual smoke checklist done (or explicit user sign-off).
