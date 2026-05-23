# Merge Self-signed Certificate into Issue Certificate Implementation Plan

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** Collapse the certificate list header from three actions to two by absorbing the standalone "Self-signed Certificate" entry into the existing "Issue Certificate" dialog as a third Certificate Type option.

**Architecture:** Two-file frontend refactor. `DNSIssueCertificate.vue` gains a `'self_signed'` value for its `certType` state, renders `SelfSignedCertFields` instead of the ACME form when selected, and calls `cert.generate_self_signed()` on submit. `CertificateList/Certificate.vue` drops the standalone button, ref, handler, and `SelfSignedCertForm` mount.

**Tech Stack:** Vue 3 (`<script setup lang="ts">`), Ant Design Vue (`AModal`, `AForm`, `AButton`, `ASelect`), the project's `$gettext` i18n helper, and the existing `cert` API client at `app/src/api/cert.ts`.

**Spec:** [`docs/superpowers/specs/2026-05-23-merge-self-signed-into-issue-cert-design.md`](../specs/2026-05-23-merge-self-signed-into-issue-cert-design.md)

---

## Files touched

- Modify: `app/src/views/certificate/components/DNSIssueCertificate.vue`
- Modify: `app/src/views/certificate/CertificateList/Certificate.vue`

Untouched (deliberately — see spec):

- `app/src/views/certificate/components/SelfSignedCertForm.vue` (still used by site editor)
- `app/src/views/certificate/components/SelfSignedCertFields.vue` (reused as the field set)
- `app/src/views/certificate/components/SelfSignedCertManagement.vue`
- `app/src/views/site/site_edit/components/Cert/SelfSignedCert.vue`

No backend changes, no Go tests.

---

## Project conventions reminder

- Frontend stack: pnpm only, Vue 3 Composition API with `<script setup>`, TypeScript, Ant Design Vue, UnoCSS.
- Code quality gates: `pnpm lint`, `pnpm lint:fix`, `pnpm typecheck` must all pass before commit.
- Vue auto-imports are configured in this project: `ref`, `computed`, `watch`, `App`, `$gettext` etc. don't need explicit imports — match the surrounding file's style.
- Comments must be in English.
- Commits: short imperative subject; sign off with `Co-Authored-By: Claude Opus 4.7 (1M context) <noreply@anthropic.com>` (project convention).

---

### Task 1: Extend DNSIssueCertificate with self-signed support

**Files:**
- Modify: `app/src/views/certificate/components/DNSIssueCertificate.vue`

Two cooperating template branches need to coexist:

1. The existing `template v-else` for `customDomains` currently triggers on any value that isn't `'wildcard'`. After we add `'self_signed'`, that `v-else` would wrongly render the Custom Domains input for self-signed mode. We must change it to `v-else-if="certType === 'custom'"`.
2. The `AutoCertForm` + Next button block must hide when `certType === 'self_signed'`.
3. A new `SelfSignedCertFields` + Generate button block must render when `certType === 'self_signed'`.

- [ ] **Step 1: Update script imports and state**

Replace the entire `<script setup lang="ts">` block (lines 1–105) with this:

```vue
<script setup lang="ts">
import type { Ref } from 'vue'
import type { AutoCertOptions } from '@/api/auto_cert'
import type { SelfSignedCertPayload } from '@/api/cert'
import AutoCertForm from '@/components/AutoCertForm'
import cert from '@/api/cert'
import { PrivateKeyTypeEnum } from '@/constants'
import ObtainCertLive from '@/views/site/site_edit/components/Cert/ObtainCertLive.vue'
import SelfSignedCertFields from './SelfSignedCertFields.vue'

const emit = defineEmits<{
  issued: [void]
}>()

const { message } = App.useApp()

type CertType = 'wildcard' | 'custom' | 'self_signed'

const step = ref(0)
const visible = ref(false)
const data = ref({}) as Ref<AutoCertOptions>
const domain = ref('')
const certType = ref<CertType>('wildcard')
const customDomains = ref<string[]>([''])
const errored = ref(false)
const selfSignedLoading = ref(false)

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

const selfSignedPayload = ref<SelfSignedCertPayload>(emptySelfSignedPayload())

function open() {
  visible.value = true
  step.value = 0
  data.value = {
    challenge_method: 'dns01',
    key_type: 'P256',
  } as AutoCertOptions
  domain.value = ''
  certType.value = 'wildcard'
  customDomains.value = ['']
  errored.value = false
  selfSignedPayload.value = emptySelfSignedPayload()
}

defineExpose({
  open,
})

const modalVisible = ref(false)
const modalClosable = ref(true)

const refObtainCertLive = useTemplateRef('refObtainCertLive')

const computedDomain = computed(() => {
  return `*.${domain.value}`
})

const computedDomains = computed(() => {
  if (certType.value === 'wildcard') {
    return [computedDomain.value, domain.value]
  }
  else {
    return customDomains.value.filter(d => d.trim())
  }
})

const computedMainDomain = computed(() => {
  if (certType.value === 'wildcard') {
    return computedDomain.value
  }
  else {
    return customDomains.value.find(d => d.trim()) || ''
  }
})

function addCustomDomain() {
  customDomains.value.push('')
}

function removeCustomDomain(index: number) {
  if (customDomains.value.length > 1) {
    customDomains.value.splice(index, 1)
  }
}

function issueCert() {
  if (!data.value.dns_credential_id) {
    message.error($gettext('Please select a DNS credential'))
    return
  }

  if (certType.value === 'custom') {
    const validDomains = customDomains.value.filter(d => d.trim())
    if (validDomains.length === 0) {
      message.error($gettext('Please enter at least one domain'))
      return
    }
  }

  errored.value = false
  step.value = 1
  modalVisible.value = true

  // ObtainCertLive is mounted in the same modal via force-render, so the
  // ref is guaranteed to be available by the time this function runs.
  refObtainCertLive.value!
    .issue_cert(computedMainDomain.value, computedDomains.value, data.value.key_type)
    .then(() => {
      message.success($gettext('Issued successfully'))
      emit('issued')
    })
    .catch(() => {
      errored.value = true
    })
}

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
</script>
```

Key script-side changes vs. the original:

- Added imports: `SelfSignedCertPayload` type, `cert` API client, `PrivateKeyTypeEnum` constant, `SelfSignedCertFields` component.
- Added `CertType` union, `selfSignedLoading`, `emptySelfSignedPayload()`, `selfSignedPayload` ref.
- `open()` resets `selfSignedPayload`.
- New `submitSelfSigned()` function.

- [ ] **Step 2: Update the Certificate Type select**

In the `<template>` block, modify the `<ASelect v-model:value="certType">` (lines ~122–129) to add the third option. Replace:

```vue
            <ASelect v-model:value="certType">
              <ASelectOption value="wildcard">
                {{ $gettext('Wildcard Certificate') }}
              </ASelectOption>
              <ASelectOption value="custom">
                {{ $gettext('Custom Domains Certificate') }}
              </ASelectOption>
            </ASelect>
```

with:

```vue
            <ASelect v-model:value="certType">
              <ASelectOption value="wildcard">
                {{ $gettext('Wildcard Certificate') }}
              </ASelectOption>
              <ASelectOption value="custom">
                {{ $gettext('Custom Domains Certificate') }}
              </ASelectOption>
              <ASelectOption value="self_signed">
                {{ $gettext('Self-signed Certificate') }}
              </ASelectOption>
            </ASelect>
```

- [ ] **Step 3: Scope the custom-domain branch to `'custom'` only**

The current `<template v-else>` (line ~142) will fire for `'self_signed'` too — change it to an explicit conditional. Replace:

```vue
          <template v-else>
            <AFormItem :label="$gettext('Custom Domains')">
```

with:

```vue
          <template v-else-if="certType === 'custom'">
            <AFormItem :label="$gettext('Custom Domains')">
```

(Only the opening tag changes; nothing else in that branch needs editing.)

- [ ] **Step 4: Hide AutoCertForm + Next button in self-signed mode and add the self-signed field set + Generate button**

Find this block (originally lines ~183–200):

```vue
        <AutoCertForm
          v-model:options="data"
          style="max-width: 600px"
          hide-note
          force-dns-challenge
        />

        <div
          v-if="step === 0"
          class="flex justify-end"
        >
          <AButton
            type="primary"
            @click="issueCert"
          >
            {{ $gettext('Next') }}
          </AButton>
        </div>
```

Replace it with:

```vue
        <template v-if="certType !== 'self_signed'">
          <AutoCertForm
            v-model:options="data"
            style="max-width: 600px"
            hide-note
            force-dns-challenge
          />

          <div class="flex justify-end">
            <AButton
              type="primary"
              @click="issueCert"
            >
              {{ $gettext('Next') }}
            </AButton>
          </div>
        </template>

        <template v-else>
          <SelfSignedCertFields v-model="selfSignedPayload" />

          <div class="flex justify-end">
            <AButton
              type="primary"
              :loading="selfSignedLoading"
              @click="submitSelfSigned"
            >
              {{ $gettext('Generate') }}
            </AButton>
          </div>
        </template>
```

Two intentional simplifications versus the original:
- Dropped the inner `v-if="step === 0"` on the Next-button div — the whole block is already inside `<template v-if="step === 0">` at the outer level (line ~119), so the inner guard was always true.
- Generate button has no surrounding `v-if="step === 0"` for the same reason.

- [ ] **Step 5: Verify lint and types**

Run from `app/`:

```bash
pnpm lint && pnpm typecheck
```

Expected: both commands exit 0. If `pnpm lint` flags style issues, run `pnpm lint:fix` and re-run `pnpm lint`.

- [ ] **Step 6: Commit**

```bash
git add app/src/views/certificate/components/DNSIssueCertificate.vue
git commit -m "$(cat <<'EOF'
feat(cert): add self-signed option in issue certificate dialog

Extend the Issue Certificate dialog's Certificate Type select with a
"Self-signed" option that swaps the form body to SelfSignedCertFields
and routes submission through cert.generate_self_signed(). ACME paths
(Wildcard / Custom Domains) are unchanged.

Co-Authored-By: Claude Opus 4.7 (1M context) <noreply@anthropic.com>
EOF
)"
```

---

### Task 2: Remove the standalone Self-signed Certificate button from the list header

**Files:**
- Modify: `app/src/views/certificate/CertificateList/Certificate.vue`

After Task 1 the Issue Certificate dialog already covers the self-signed flow. Now we remove the duplicate header entry.

- [ ] **Step 1: Replace the file's `<script setup>` block**

Replace lines 1–25 (the entire `<script setup lang="tsx">` block) with:

```vue
<script setup lang="tsx">
import { CloudUploadOutlined, SafetyCertificateOutlined } from '@ant-design/icons-vue'
import { StdTable } from '@uozi-admin/curd'
import cert from '@/api/cert'
import { useGlobalStore } from '@/pinia'
import WildcardCertificate from '../components/DNSIssueCertificate.vue'
import RemoveCert from '../components/RemoveCert.vue'
import RetryCert from '../components/RetryCert.vue'
import certColumns from './certColumns'

const refWildcard = ref()
const refTable = ref()

const globalStore = useGlobalStore()

const { processingStatus } = storeToRefs(globalStore)
</script>
```

Removed:
- `import type { Cert } from '@/api/cert'` (only `onSelfSignedCreated` used it).
- `SafetyOutlined` from the icons import (drop just that name, keep the other two).
- `SelfSignedCertForm` import.
- `refSelfSigned` ref.
- `router` and `onSelfSignedCreated` (no longer needed — the dialog's `issued` emit drives the refresh).

- [ ] **Step 2: Remove the Self-signed button and modal mount from the template**

Replace the `<template>` block (lines 27–91) with:

```vue
<template>
  <ACard :title="$gettext('Certificates')">
    <template #extra>
      <AButton
        type="link"
        size="small"
        @click="$router.push('/certificates/import')"
      >
        <CloudUploadOutlined />
        {{ $gettext('Import') }}
      </AButton>

      <AButton
        type="link"
        size="small"
        :disabled="processingStatus.auto_cert_processing"
        @click="() => refWildcard.open()"
      >
        <SafetyCertificateOutlined />
        {{ $gettext('Issue certificate') }}
      </AButton>
    </template>
    <StdTable
      ref="refTable"
      :api="cert"
      :columns="certColumns"
      :get-list-api="cert.getList"
      disable-view
      :scroll-x="1000"
      disable-delete
      @edit-item="record => $router.push(`/certificates/${record.id}`)"
    >
      <template #afterActions="{ record }">
        <RetryCert
          v-if="record.status === 'failure'"
          :cert="record"
          @retried="() => refTable.refresh()"
        />
        <RemoveCert
          :id="record.id"
          :certificate="record"
          :disabled="processingStatus.auto_cert_processing"
          @removed="() => refTable.refresh()"
        />
      </template>
    </StdTable>
    <WildcardCertificate
      ref="refWildcard"
      @issued="() => refTable.refresh()"
    />
  </ACard>
</template>
```

Removed:
- The `<AButton>` with `<SafetyOutlined />` and the Self-signed Certificate text.
- The `<SelfSignedCertForm ref="refSelfSigned" @created="onSelfSignedCreated" />` block at the bottom.

The `<style lang="less" scoped>` block at the end of the file is unchanged.

- [ ] **Step 3: Verify lint and types**

Run from `app/`:

```bash
pnpm lint && pnpm typecheck
```

Expected: both exit 0. The removed imports must be cleanly removed — leftover `Cert` or `SafetyOutlined` references would surface as lint warnings here.

- [ ] **Step 4: Commit**

```bash
git add app/src/views/certificate/CertificateList/Certificate.vue
git commit -m "$(cat <<'EOF'
refactor(cert): drop standalone self-signed button from list header

Certificate creation is now consolidated under the Issue Certificate
dialog (which exposes Self-signed as a Certificate Type option), so
the duplicate header entry, its ref, handler, and modal mount are
removed.

Co-Authored-By: Claude Opus 4.7 (1M context) <noreply@anthropic.com>
EOF
)"
```

---

### Task 3: Manual verification

**Files:** none modified.

These are the steps to convince yourself the change works end-to-end before declaring done. No automation; the project doesn't carry frontend component tests for this view.

- [ ] **Step 1: Start the frontend dev server**

From `app/`:

```bash
pnpm install
pnpm run dev
```

Expected: Vite reports ready and prints a local URL.

- [ ] **Step 2: Open the certificate list page**

Navigate to the Certificates page in the running UI.

Expected: the card header shows exactly two action buttons: **Import** and **Issue certificate**. The standalone Self-signed Certificate button is gone.

- [ ] **Step 3: Verify Wildcard ACME flow still works (no regression)**

Click **Issue certificate**. The Certificate Type select defaults to `Wildcard Certificate`. Fill in a domain, pick a DNS credential, click **Next**. The dialog should advance to the ObtainCertLive step exactly as before.

(You don't need to actually complete an ACME issuance unless you have a credential handy — the goal is just to confirm the form still renders and Next triggers the live step.)

- [ ] **Step 4: Verify Custom Domains flow still works (no regression)**

Re-open the dialog, switch the select to `Custom Domains Certificate`. Confirm the custom-domain list, Add Domain button, and Remove buttons all render. The DNS provider alert below should still show.

- [ ] **Step 5: Verify the new Self-signed flow**

Re-open the dialog, switch the select to `Self-signed Certificate`.

Expected:
- The ACME form (Challenge Method, ACME User, DNS provider, OCSP, Revoke Old) disappears.
- The self-signed field set appears: Name, Domains (tag input), IP Addresses (tag input), Key Type, Valid For (days), Sync to.
- The bottom button reads **Generate**.

Click **Generate** with both Domains and IP Addresses empty.

Expected: red error toast "Please enter at least one domain or IP address". Dialog stays open.

Enter at least one domain (e.g. `local.test`) and click **Generate**.

Expected: green toast "Self-signed certificate generated". Dialog closes. The new row appears in the table.

- [ ] **Step 6: Verify site editor self-signed shortcut still works (regression)**

Open any site in the editor. In the Cert panel, click the self-signed shortcut (`SelfSignedCert.vue`).

Expected: its own modal opens and works exactly as before — the refactor must not have affected this code path.

- [ ] **Step 7: No commit**

Manual verification produces no files to commit.

---

## Wrap-up checklist

After both code commits land and manual verification passes:

- [ ] `pnpm lint` and `pnpm typecheck` both green on the final tree.
- [ ] `git status` is clean.
- [ ] The two commits are scoped one per task (creation extension, then header cleanup) and reviewable independently.
- [ ] No backend files touched; no Go tests need re-running.
