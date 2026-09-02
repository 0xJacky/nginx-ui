<script setup lang="ts">
import type { HostKeyScanItem, HostKeyScanResult, SetupParams } from '@/api/host_setup'
import { ScanOutlined } from '@ant-design/icons-vue'
import { computed, onActivated, ref, watch } from 'vue'
import hostSetup from '@/api/host_setup'
import { getErrorMessage } from '@/lib/http'
import CodeBlock from '../CodeBlock.vue'
import { parseHostAddress } from '../hostAddress'

const props = defineProps<{ params: SetupParams }>()
const emit = defineEmits<{ invalidated: [] }>()
const trusted = defineModel<boolean>('trusted', { default: false })

const result = ref<HostKeyScanResult | null>(null)
const scanning = ref(false)
const manualOutput = ref('')
const scanError = ref('')
const confirmed = ref<Record<string, boolean>>({})
const operating = ref<Record<string, boolean>>({})
const lastScanUsedManual = ref(false)
const lastScannedHostAddress = ref('')
const operationError = ref('')
// A scan invalidated by an address edit must not attribute its result to the
// new host when it lands.
let scanID = 0

const hasChangedKey = computed(() => result.value?.keys.some(key => key.status === 'changed') ?? false)
// A revoked key is refused at dial time, so trusting it would only produce an
// obscure handshake failure later.
const hasRevokedKey = computed(() => result.value?.keys.some(key => key.status === 'revoked') ?? false)
const hasOnlyTrustedKeys = computed(() => {
  const keys = result.value?.keys ?? []
  return keys.length > 0 && keys.every(key => key.status === 'trusted')
})

watch([hasChangedKey, hasRevokedKey, hasOnlyTrustedKeys], ([hasChanged, hasRevoked, onlyTrusted]) => {
  trusted.value = !hasChanged && !hasRevoked && onlyTrusted
}, { immediate: true })

function keyID(key: HostKeyScanItem) {
  return `${key.algorithm}:${key.fingerprint}`
}

function shellQuote(value: string) {
  return `'${value.replaceAll('\'', '\'"\'"\'')}'`
}

function sshKeyscanCommand() {
  const { host, port } = parseHostAddress(props.params.host_address)
  return `ssh-keyscan -p ${shellQuote(port)} ${shellQuote(host)}`
}

function statusColor(status: HostKeyScanItem['status']) {
  switch (status) {
    case 'trusted':
      return 'success'
    case 'new_algorithm':
      return 'processing'
    case 'changed':
      return 'error'
    case 'stale':
      return 'warning'
    case 'revoked':
      return 'error'
    default:
      return 'warning'
  }
}

function statusText(status: HostKeyScanItem['status']) {
  switch (status) {
    case 'trusted':
      return $gettext('Trusted')
    case 'new_algorithm':
      return $gettext('New algorithm')
    case 'changed':
      return $gettext('Changed')
    case 'stale':
      return $gettext('Stale')
    case 'revoked':
      return $gettext('Revoked')
    default:
      return $gettext('Unknown host')
  }
}

async function scan(useManual = false) {
  const address = props.params.host_address
  const currentScan = ++scanID
  scanning.value = true
  trusted.value = false
  emit('invalidated')
  operationError.value = ''
  scanError.value = ''
  result.value = null
  lastScanUsedManual.value = useManual
  try {
    const scanned = await hostSetup.scanHostKeys({
      host_address: address,
      keyscan_output: useManual ? manualOutput.value : undefined,
    })
    if (currentScan !== scanID)
      return
    result.value = scanned
    lastScannedHostAddress.value = address
  }
  catch (error) {
    if (currentScan !== scanID)
      return
    scanError.value = getErrorMessage(error)
  }
  finally {
    // A stale run must not clear the loading state of a newer scan.
    if (currentScan === scanID)
      scanning.value = false
  }
}

// Pasted ssh-keyscan output describes one host. Keep it from being re-submitted
// for a different address.
watch(() => props.params.host_address, () => {
  scanID++
  manualOutput.value = ''
  lastScanUsedManual.value = false
  result.value = null
  lastScannedHostAddress.value = ''
  trusted.value = false
})

async function refresh() {
  await scan(lastScanUsedManual.value)
}

async function trust(key: HostKeyScanItem) {
  const id = keyID(key)
  operating.value[id] = true
  operationError.value = ''
  try {
    await hostSetup.trustScannedHostKey({
      host_address: props.params.host_address,
      algorithm: key.algorithm,
      fingerprint: key.fingerprint,
      public_key: key.public_key,
      confirmed: true,
    })
    await refresh()
  }
  catch (error) {
    operationError.value = getErrorMessage(error)
  }
  finally {
    operating.value[id] = false
  }
}

async function replace(key: HostKeyScanItem) {
  const id = keyID(key)
  operating.value[id] = true
  operationError.value = ''
  try {
    await hostSetup.replaceHostKey({
      host_address: props.params.host_address,
      algorithm: key.algorithm,
      old_fingerprint: key.existing_fingerprint ?? '',
      new_fingerprint: key.fingerprint,
      public_key: key.public_key,
      confirmed: true,
    })
    await refresh()
  }
  catch (error) {
    operationError.value = getErrorMessage(error)
  }
  finally {
    operating.value[id] = false
  }
}

async function deleteStale(key: HostKeyScanItem) {
  const id = keyID(key)
  operating.value[id] = true
  operationError.value = ''
  try {
    await hostSetup.deleteHostKey({
      host_address: props.params.host_address,
      algorithm: key.algorithm,
      fingerprint: key.fingerprint,
      confirmed: true,
    })
    await refresh()
  }
  catch (error) {
    operationError.value = getErrorMessage(error)
  }
  finally {
    operating.value[id] = false
  }
}

onActivated(() => {
  if (!result.value || lastScannedHostAddress.value !== props.params.host_address) {
    void scan(lastScanUsedManual.value)
    return
  }
  // The wizard clears `trusted` whenever the address is edited, even when the
  // edit is reverted. Re-derive it from the cached scan so the model and the
  // rendered result cannot disagree.
  trusted.value = !hasChangedKey.value && !hasRevokedKey.value && hasOnlyTrustedKeys.value
})
</script>

<template>
  <div class="space-y-4">
    <AAlert
      type="warning"
      show-icon
      :message="$gettext('Verify the SSH host key before trusting it')"
      :description="$gettext('Nginx UI can read the key presented by the SSH server, but it cannot prove the key is genuine by itself. Verify the fingerprint through the host console or another trusted channel before trusting or replacing it.')"
    />

    <ACard size="small" :title="$gettext('Current target')">
      <ADescriptions bordered size="small" :column="1">
        <ADescriptionsItem :label="$gettext('Host')">
          <span class="break-all">{{ params.host_address }}</span>
        </ADescriptionsItem>
        <ADescriptionsItem :label="$gettext('SSH user')">
          <span class="break-all">{{ params.host_user }}</span>
        </ADescriptionsItem>
        <ADescriptionsItem v-if="result" :label="$gettext('known_hosts')">
          <span class="break-all">{{ result.known_hosts_path }}</span>
        </ADescriptionsItem>
      </ADescriptions>
      <AAlert
        v-if="result?.persistence?.warning"
        type="warning"
        show-icon
        class="mt-3"
        :message="result.persistence.warning"
      />
    </ACard>

    <ADivider orientation="left">
      {{ $gettext('1. Verify and trust the host key') }}
    </ADivider>

    <ASpace wrap>
      <AButton type="primary" :loading="scanning" @click="scan(false)">
        <ScanOutlined />
        {{ $gettext('Scan host keys') }}
      </AButton>
    </ASpace>

    <AAlert
      v-if="scanError"
      type="error"
      show-icon
      :message="$gettext('Failed to scan host keys')"
      :description="scanError"
    />
    <AAlert
      v-if="operationError"
      type="error"
      show-icon
      :message="$gettext('Failed to update trusted host keys')"
      :description="operationError"
    />

    <ACollapse>
      <ACollapsePanel key="manual" :header="$gettext('Paste ssh-keyscan output')">
        <CodeBlock :code="sshKeyscanCommand()" language="shell" :title="$gettext('Run on a trusted terminal')" />
        <ATextarea v-model:value="manualOutput" class="mt-3" :rows="4" />
        <AButton class="mt-3" :disabled="!manualOutput" :loading="scanning" @click="scan(true)">
          {{ $gettext('Parse pasted output') }}
        </AButton>
      </ACollapsePanel>
    </ACollapse>

    <AList v-if="result" :data-source="result.keys">
      <template #renderItem="{ item }">
        <AListItem>
          <ACard class="w-full" size="small">
            <div class="flex flex-wrap items-center justify-between gap-2">
              <strong>{{ item.algorithm }}</strong>
              <ATag :color="statusColor(item.status)" :bordered="false">
                {{ statusText(item.status) }}
              </ATag>
            </div>
            <p class="mt-2 break-all">
              <strong>{{ $gettext('Fingerprint') }}:</strong> {{ item.fingerprint }}
            </p>
            <p v-if="item.existing_fingerprint" class="break-all">
              <strong>{{ $gettext('Existing fingerprint') }}:</strong> {{ item.existing_fingerprint }}
            </p>
            <ACollapse class="mt-2">
              <ACollapsePanel key="pub" :header="$gettext('Public key')">
                <ATypographyParagraph
                  class="mb-0 break-all"
                  code
                  :copyable="{ text: item.public_key, tooltip: false }"
                >
                  {{ item.public_key }}
                </ATypographyParagraph>
              </ACollapsePanel>
            </ACollapse>

            <div v-if="item.status === 'unknown_host' || item.status === 'new_algorithm'" class="mt-3 space-y-2">
              <ACheckbox v-model:checked="confirmed[keyID(item)]">
                {{ $gettext('I verified this fingerprint through a trusted channel.') }}
              </ACheckbox>
              <AButton :disabled="!confirmed[keyID(item)]" :loading="operating[keyID(item)]" @click="trust(item)">
                {{ item.status === 'new_algorithm' ? $gettext('Trust this algorithm') : $gettext('Trust this key') }}
              </AButton>
            </div>

            <div v-if="item.status === 'changed'" class="mt-3 space-y-2">
              <AAlert type="error" show-icon :message="$gettext('Host key changed. Replace only after confirming an intentional host SSH key rotation.')" />
              <ACheckbox v-model:checked="confirmed[keyID(item)]">
                {{ $gettext('I verified the new fingerprint through a trusted channel.') }}
              </ACheckbox>
              <AButton danger :disabled="!confirmed[keyID(item)]" :loading="operating[keyID(item)]" @click="replace(item)">
                {{ $gettext('Replace trusted key') }}
              </AButton>
            </div>
          </ACard>
        </AListItem>
      </template>
    </AList>

    <AEmpty
      v-if="result && result.keys.length === 0"
      :description="$gettext('No SSH host keys were returned')"
    />

    <ACollapse v-if="result?.stale_keys?.length">
      <ACollapsePanel key="stale" :header="$gettext('Advanced cleanup')">
        <AList :data-source="result.stale_keys">
          <template #renderItem="{ item }">
            <AListItem>
              <div class="w-full">
                <ATag color="warning" :bordered="false">
                  {{ $gettext('Stale') }}
                </ATag>
                <strong class="ml-2">{{ item.algorithm }}</strong>
                <div class="text-secondary mt-1 break-all text-sm">
                  {{ item.fingerprint }}
                </div>
                <ACheckbox v-model:checked="confirmed[keyID(item)]" class="mt-2">
                  {{ $gettext('I understand this removes only the selected stale known_hosts entry.') }}
                </ACheckbox>
                <div>
                  <AButton class="mt-2" danger size="small" :disabled="!confirmed[keyID(item)]" :loading="operating[keyID(item)]" @click="deleteStale(item)">
                    {{ $gettext('Delete stale key') }}
                  </AButton>
                </div>
              </div>
            </AListItem>
          </template>
        </AList>
      </ACollapsePanel>
    </ACollapse>

    <AAlert
      v-if="result && !hasChangedKey && !hasRevokedKey && hasOnlyTrustedKeys"
      type="success"
      show-icon
      :message="$gettext('Host identity is trusted')"
    />
    <AAlert
      v-if="hasChangedKey"
      type="error"
      show-icon
      :message="$gettext('Resolve changed host keys before continuing.')"
    />
    <AAlert
      v-if="hasRevokedKey"
      type="error"
      show-icon
      :message="$gettext('The host presented a revoked key')"
      :description="$gettext('This key is marked @revoked in known_hosts. SSH will refuse it, so trusting it here would only fail later. Investigate why the host is presenting a revoked key before continuing.')"
    />
  </div>
</template>
