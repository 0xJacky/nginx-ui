<script setup lang="ts">
import type { Ref } from 'vue'
import type { AutoCertOptions } from '@/api/auto_cert'
import type { Cert, SelfSignedCertPayload } from '@/api/cert'
import { EditOutlined, EllipsisOutlined } from '@antdv-next/icons'
import cert, { toSelfSignedPayload } from '@/api/cert'
import { AutoCertState, normalizePrivateKeyType } from '@/constants'
import { getErrorMessage } from '@/lib/http'
import { useGlobalStore } from '@/pinia'
import { collectChangedPaths } from '@/utils/changedPaths'
import { PreferenceSaveBar } from '@/views/preference/components/Shell'
import { isAcmeCert } from './certState'
import AutoCertManagement from './components/AutoCertManagement.vue'
import CertificateActions from './components/CertificateActions.vue'
import CertificateDeployTargets from './components/CertificateDeployTargets.vue'
import CertificateDownload from './components/CertificateDownload.vue'
import CertificateFacts from './components/CertificateFacts.vue'
import CertificateFilesCard from './components/CertificateFilesCard.vue'
import CertificateLastRenewal from './components/CertificateLastRenewal.vue'
import CertificateSyncCard from './components/CertificateSyncCard.vue'
import RemoveCert from './components/RemoveCert.vue'
import RenewCert from './components/RenewCert.vue'
import SelfSignedCertManagement from './components/SelfSignedCertManagement.vue'
import { useCertStore } from './store'

const { message } = useGlobalApp()

const route = useRoute()
const certStore = useCertStore()
const router = useRouter()
const globalStore = useGlobalStore()
const { processingStatus } = storeToRefs(globalStore)
const errors = ref({}) as Ref<Record<string, string>>

const id = computed(() => Number.parseInt(route.params.id as string))
const isNew = computed(() => !(id.value > 0))

const { data } = storeToRefs(certStore)

// Renewal and file contents are handled by Nginx UI for these.
const isManaged = computed(() => data.value.auto_cert === AutoCertState.Enable
  || data.value.auto_cert === AutoCertState.Paused
  || data.value.auto_cert === AutoCertState.Sync)

const isSelfSigned = computed(() => data.value.auto_cert === AutoCertState.SelfSigned)
const isAcme = computed(() => !isNew.value && isAcmeCert(data.value))

const canRenew = computed(() => isAcme.value && (data.value.auto_cert === AutoCertState.Enable
  || data.value.auto_cert === AutoCertState.Paused))

const canSwitchAutoRenewal = computed(() => isAcme.value && !!data.value.domains?.length
  && [AutoCertState.Enable, AutoCertState.Disable, AutoCertState.Paused].includes(data.value.auto_cert))

const selfSignedPayload = ref<SelfSignedCertPayload>()

// ---- Change tracking ------------------------------------------------------

const trackedFields = {
  name: () => $gettext('Name'),
  challenge_method: () => $gettext('Challenge Method'),
  dns_credential_id: () => $gettext('DNS Credential'),
  key_type: () => $gettext('Key Type'),
  acme_user_id: () => $gettext('ACME User'),
  challenge_config: () => $gettext('Advanced options'),
  must_staple: () => $gettext('OCSP Must Staple'),
  enable_common_name: () => $gettext('Enable Common Name'),
  revoke_old: () => $gettext('Revoke Old Certificate'),
  sync_node_ids: () => $gettext('Sync to other nodes'),
  ssl_certificate: () => $gettext('Certificate'),
  ssl_certificate_key: () => $gettext('Private key'),
} satisfies Partial<Record<keyof Cert, () => string>>

const selfSignedFields = {
  name: () => $gettext('Name'),
  domains: () => $gettext('Domains'),
  ip_addresses: () => $gettext('IP Addresses'),
  key_type: () => $gettext('Key Type'),
  validity_days: () => $gettext('Valid For (days)'),
  sync_node_ids: () => $gettext('Sync to other nodes'),
} satisfies Record<keyof SelfSignedCertPayload, () => string>

function clone<T>(value: T): T {
  return value === undefined ? value : JSON.parse(JSON.stringify(value))
}

function pickTracked(source: Partial<Cert>) {
  const picked: Record<string, unknown> = {}
  for (const key of Object.keys(trackedFields))
    picked[key] = clone(source[key as keyof Cert])

  // The credential id is tracked on its own; its copy in the challenge
  // options follows it.
  const config = picked.challenge_config as Record<string, unknown> | undefined
  if (config) {
    delete config.credential_id
    if (Object.keys(config).length === 0)
      picked.challenge_config = undefined
  }
  return picked
}

const baseline = ref<Record<string, unknown>>({})
const selfSignedBaseline = ref<SelfSignedCertPayload>()

// Forms fill defaults into the challenge options right after they mount.
// Those writes are part of loading, not edits.
let absorbUntil = 0

function markSaved() {
  baseline.value = pickTracked(data.value)
  selfSignedBaseline.value = clone(selfSignedPayload.value)
  absorbUntil = Date.now() + 1500
}

watch(() => data.value.challenge_config, config => {
  if (Date.now() < absorbUntil)
    baseline.value = { ...baseline.value, challenge_config: pickTracked({ challenge_config: config }).challenge_config }
}, { deep: true })

function labelsOf<T extends Record<string, () => string>>(paths: string[], labels: T) {
  const result = paths
    .map(path => labels[path.split('.')[0] as keyof T]?.())
    .filter((label): label is string => !!label)
  return [...new Set(result)]
}

const changedLabels = computed(() => {
  if (isNew.value)
    return []

  if (isSelfSigned.value)
    return labelsOf(collectChangedPaths(selfSignedBaseline.value ?? {}, selfSignedPayload.value ?? {}), selfSignedFields)

  return labelsOf(collectChangedPaths(baseline.value, pickTracked(data.value)), trackedFields)
})

// ---- Loading ------------------------------------------------------------

function normalizeLoaded(r: Cert): Cert {
  return {
    ...r,
    // Backend stores key_type in its canonical form (EC256, RSA2048…); the
    // ACME form's ASelect options use the legacy keys (P256, 2048…).
    key_type: normalizePrivateKeyType(r.key_type),
    // An id of 0 means no credential.
    dns_credential_id: r.dns_credential_id || (undefined as unknown as number),
  }
}

function applyLoaded(r: Cert) {
  data.value = normalizeLoaded(r)
  selfSignedPayload.value = r.auto_cert === AutoCertState.SelfSigned ? toSelfSignedPayload(r) : undefined
  markSaved()
}

// The store is a singleton, so editing another certificate starts out holding
// the previous one's data. Every load takes a ticket so a slow response cannot
// land after the operator moved on to another record.
let loadSeq = 0

async function load() {
  const seq = ++loadSeq
  const target = id.value

  // Keep the page filled when this is a reload of the record already on
  // screen, e.g. after a renewal.
  if (data.value.id !== target) {
    data.value = {} as Cert
    selfSignedPayload.value = undefined
    baseline.value = {}
  }

  if (!(target > 0))
    return

  const r = await cert.getItem(target)
  if (seq === loadSeq)
    applyLoaded(r)
}

// Vue Router can reuse this component when only the id changes, so reload on
// the route parameter instead of on mount alone.
watch(id, load, { immediate: true })

// ---- Header ------------------------------------------------------------

const displayName = computed(() => {
  if (isSelfSigned.value && selfSignedPayload.value)
    return selfSignedPayload.value.name
  return data.value.name || data.value.certificate_info?.subject_name || ''
})

const domains = computed(() => {
  if (data.value.domains?.length)
    return data.value.domains
  const info = data.value.certificate_info
  if (!info)
    return []
  return [info.subject_name, ...(info.subject_alt_names ?? [])].filter(Boolean)
})

// The name of an ACME certificate follows its domains and is rewritten on
// renewal, so only other certificates can be renamed.
const canRename = computed(() => !isNew.value && !isManaged.value)

const renaming = ref(false)
const nameDraft = ref('')
function startRename() {
  nameDraft.value = displayName.value
  renaming.value = true
}

function finishRename() {
  if (!renaming.value)
    return
  renaming.value = false
  const name = nameDraft.value.trim()
  if (!name || name === displayName.value)
    return
  if (isSelfSigned.value && selfSignedPayload.value)
    selfSignedPayload.value.name = name
  else
    data.value.name = name
}

function cancelRename() {
  renaming.value = false
}

const isDelegated = computed(() => !!data.value.delegated_node_id)
const delegatedNode = computed(() => data.value.delegated_node_name || `#${data.value.delegated_node_id}`)

const renewOptions = computed<AutoCertOptions>(() => ({
  // A certificate issued for a node is renewed for its configuration there.
  name: isDelegated.value
    ? data.value.delegated_config_name || data.value.name
    : data.value.filename || data.value.name,
  delegated_node_id: data.value.delegated_node_id || undefined,
  domains: data.value.domains,
  key_type: data.value.key_type,
  challenge_method: data.value.challenge_method,
  profile: data.value.profile,
  dns_credential_id: data.value.dns_credential_id,
  acme_user_id: data.value.acme_user_id,
  must_staple: data.value.must_staple,
  challenge_config: data.value.challenge_config,
  enable_common_name: data.value.enable_common_name,
  revoke_old: data.value.revoke_old,
}))

const removeCert = useTemplateRef('removeCert')

const moreMenu = computed(() => ({
  items: [{ key: 'delete', danger: true, label: $gettext('Delete'), disabled: processingStatus.value.auto_cert_processing }],
  onClick: ({ key }: { key: string | number }) => {
    if (key === 'delete')
      removeCert.value?.open()
  },
}))

function handleRemoved() {
  router.push('/certificates/list')
}

// ---- Auto renewal ---------------------------------------------------------

const switchingAutoRenewal = ref(false)

async function switchAutoRenewal(enabled: boolean) {
  switchingAutoRenewal.value = true
  try {
    const r = await cert.set_auto_renewal(data.value.id, enabled)
    data.value.auto_cert = r.auto_cert
    data.value.state = r.state
    data.value.renew_at = r.renew_at
    message.success(enabled ? $gettext('Automatic renewal turned on') : $gettext('Automatic renewal turned off'))
  }
  catch (e) {
    message.error(getErrorMessage(e, $gettext('Server error')))
  }
  finally {
    switchingAutoRenewal.value = false
  }
}

// ---- Save ------------------------------------------------------------------

const saving = ref(false)

async function save() {
  saving.value = true
  try {
    let savedId = data.value.id
    if (isSelfSigned.value && selfSignedPayload.value && data.value.id) {
      const payload = selfSignedPayload.value
      const name = payload.name.trim()
      const domainList = payload.domains.map(d => d.trim()).filter(Boolean)
      const ip_addresses = payload.ip_addresses.map(s => s.trim()).filter(Boolean)

      if (!name) {
        message.error($gettext('Please enter a name for the certificate'))
        return
      }
      if (domainList.length === 0 && ip_addresses.length === 0) {
        message.error($gettext('Please enter at least one domain or IP address'))
        return
      }

      const result = await cert.modify_self_signed(data.value.id, {
        ...payload,
        name,
        domains: domainList,
        ip_addresses,
      })
      savedId = result.id || data.value.id
    }
    else {
      await certStore.save()
      savedId = data.value.id
    }
    if (!savedId) {
      message.error($gettext('Saved certificate response is missing an ID'))
      return
    }
    message.success($gettext('Save successfully'))
    errors.value = {}

    if (savedId !== id.value) {
      await router.push(`/certificates/${savedId}`)
      return
    }
    applyLoaded(await cert.getItem(savedId))
  }
  // eslint-disable-next-line ts/no-explicit-any
  catch (e: any) {
    errors.value = e.errors ?? {}
    message.error(e.message ?? $gettext('Server error'))
  }
  finally {
    saving.value = false
  }
}

function discard() {
  const saved = baseline.value
  for (const key of Object.keys(trackedFields))
    (data.value as unknown as Record<string, unknown>)[key] = clone(saved[key])
  // The credential id is part of the challenge options too.
  if (data.value.dns_credential_id) {
    data.value.challenge_config = {
      ...data.value.challenge_config,
      credential_id: String(data.value.dns_credential_id),
    }
  }
  selfSignedPayload.value = clone(selfSignedBaseline.value)
  errors.value = {}
}

function handleBack() {
  router.push('/certificates/list')
}
</script>

<template>
  <ACard>
    <!-- Header -->
    <div class="editor-head">
      <div class="min-w-0 flex-1">
        <template v-if="isNew">
          <h2 class="editor-title">
            {{ $gettext('Import Certificate') }}
          </h2>
          <AInput
            v-model:value="data.name"
            class="mt-3 max-w-120"
            :placeholder="$gettext('Name')"
            :status="errors.name ? 'error' : undefined"
          />
        </template>
        <template v-else>
          <AFlex align="center" wrap gap="small">
            <AInput
              v-if="renaming"
              v-model:value="nameDraft"
              autofocus
              class="max-w-120"
              @press-enter="finishRename"
              @blur="finishRename"
              @keydown.esc="cancelRename"
            />
            <h2 v-else class="editor-title">
              {{ displayName }}
            </h2>
            <AButton
              v-if="canRename && !renaming"
              type="link"
              size="small"
              class="px-0"
              @click="startRename"
            >
              <EditOutlined />
              {{ $gettext('Rename') }}
            </AButton>
          </AFlex>
          <AFlex v-if="domains.length" wrap gap="small" class="mt-2">
            <ATag v-for="domain in domains" :key="domain" class="m-0 font-mono">
              {{ domain }}
            </ATag>
          </AFlex>
        </template>
      </div>

      <AFlex v-if="!isNew" wrap gap="small" align="center" class="editor-actions">
        <CertificateDownload :data="data" plain inline />
        <RenewCert
          v-if="canRenew"
          primary
          :options="renewOptions"
          @saved="markSaved"
          @renewed="load"
        />
        <ADropdown :menu="moreMenu" :trigger="['click']" placement="bottomRight">
          <AButton :aria-label="$gettext('More actions')">
            <template #icon>
              <EllipsisOutlined />
            </template>
          </AButton>
        </ADropdown>
        <RemoveCert
          :id="data.id"
          ref="removeCert"
          :certificate="data"
          hide-trigger
          @removed="handleRemoved"
        />
      </AFlex>
    </div>

    <CertificateFacts
      v-if="!isNew && data.id"
      class="mt-4"
      :cert="data"
      :can-switch-auto-renewal="canSwitchAutoRenewal"
      :switching="switchingAutoRenewal"
      @switch-auto-renewal="switchAutoRenewal"
    />

    <ARow :gutter="[16, 16]" class="mt-4">
      <ACol :xs="24" :lg="isNew ? 24 : 15">
        <AFlex vertical :gap="16">
          <AAlert
            v-if="isDelegated"
            type="info"
            show-icon
            :title="$gettext('Issued for %{node}', { node: delegatedNode })"
          >
            <template #description>
              <p class="mb-1">
                {{ $gettext('This instance validates the domain, keeps renewing the certificate and sends each renewal to %{node}.', { node: delegatedNode }) }}
              </p>
              <p v-if="data.remote_ssl_certificate_path" class="mb-0">
                {{ $gettext('Files on the node:') }}
                <code>{{ data.remote_ssl_certificate_path }}</code>,
                <code>{{ data.remote_ssl_certificate_key_path }}</code>
              </p>
            </template>
          </AAlert>
          <SelfSignedCertManagement
            v-if="isSelfSigned && selfSignedPayload"
            v-model:value="selfSignedPayload"
          />
          <AutoCertManagement
            v-else-if="isAcme"
            v-model:data="data"
            :is-managed="isManaged"
          />

          <CertificateSyncCard
            v-if="!isSelfSigned && !isDelegated"
            v-model:sync-node-ids="data.sync_node_ids"
          />

          <CertificateFilesCard
            v-model:data="data"
            :errors="errors"
            :managed="isManaged || isSelfSigned"
          />
        </AFlex>
      </ACol>

      <ACol v-if="!isNew" :xs="24" :lg="9">
        <AFlex vertical :gap="16">
          <CertificateDeployTargets :cert-id="id" />
          <CertificateLastRenewal v-if="isAcme" :cert="data" />
        </AFlex>
      </ACol>
    </ARow>

    <CertificateActions
      v-if="isNew"
      @save="save"
      @back="handleBack"
    />
    <PreferenceSaveBar
      v-else-if="changedLabels.length"
      :labels="changedLabels"
      :saving="saving"
      @save="save"
      @discard="discard"
    />
  </ACard>
</template>

<style scoped lang="less">
.editor-head {
  display: flex;
  flex-wrap: wrap;
  align-items: flex-start;
  justify-content: space-between;
  gap: 12px 16px;
}

.editor-title {
  margin: 0;
  font-size: 20px;
  font-weight: 600;
  line-height: 32px;
  overflow-wrap: anywhere;
}

.editor-actions {
  flex: none;
}

@media (max-width: 575px) {
  .editor-actions {
    width: 100%;
  }
}
</style>
