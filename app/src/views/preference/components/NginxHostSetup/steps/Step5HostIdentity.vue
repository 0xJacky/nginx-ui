<script setup lang="ts">
import type { HostKeyScanItem, HostKeyScanResult, SetupParams } from '@/api/host_setup'
import { computed, onMounted, ref } from 'vue'
import hostSetup from '@/api/host_setup'
import CodeBlock from '../CodeBlock.vue'

const props = defineProps<{ params: SetupParams }>()

const result = ref<HostKeyScanResult | null>(null)
const scanning = ref(false)
const manualOutput = ref('')
const scanError = ref('')
const confirmed = ref<Record<string, boolean>>({})
const operating = ref<Record<string, boolean>>({})
const lastScanUsedManual = ref(false)

const hasChangedKey = computed(() => result.value?.keys.some(key => key.status === 'changed') ?? false)
const hasOnlyTrustedKeys = computed(() => {
  const keys = result.value?.keys ?? []
  return keys.length > 0 && keys.every(key => key.status === 'trusted')
})

function keyID(key: HostKeyScanItem) {
  return `${key.algorithm}:${key.fingerprint}`
}

function shellQuote(value: string) {
  return `'${value.replaceAll("'", "'\"'\"'")}'`
}

function parseHostAddress(address: string) {
  const bracketed = address.match(/^\[([^\]]+)\](?::(\d+))?$/)
  if (bracketed) {
    return {
      host: bracketed[1],
      port: bracketed[2] ?? '22',
    }
  }

  const colonCount = (address.match(/:/g) ?? []).length
  if (colonCount === 1) {
    const [host, port] = address.split(':')
    return { host, port: port || '22' }
  }

  return { host: address, port: '22' }
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
    default:
      return 'warning'
  }
}

async function scan(useManual = false) {
  scanning.value = true
  scanError.value = ''
  lastScanUsedManual.value = useManual
  try {
    result.value = await hostSetup.scanHostKeys({
      host_address: props.params.host_address,
      keyscan_output: useManual ? manualOutput.value : undefined,
    })
  }
  catch (error) {
    scanError.value = error instanceof Error ? error.message : String(error)
  }
  finally {
    scanning.value = false
  }
}

async function refresh() {
  await scan(lastScanUsedManual.value)
}

async function trust(key: HostKeyScanItem) {
  const id = keyID(key)
  operating.value[id] = true
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
  finally {
    operating.value[id] = false
  }
}

async function replace(key: HostKeyScanItem) {
  const id = keyID(key)
  operating.value[id] = true
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
  finally {
    operating.value[id] = false
  }
}

async function deleteStale(key: HostKeyScanItem) {
  const id = keyID(key)
  operating.value[id] = true
  try {
    await hostSetup.deleteHostKey({
      host_address: props.params.host_address,
      algorithm: key.algorithm,
      fingerprint: key.fingerprint,
      confirmed: true,
    })
    await refresh()
  }
  finally {
    operating.value[id] = false
  }
}

onMounted(() => {
  void scan(false)
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
      <p><strong>{{ $gettext('Host') }}:</strong> {{ params.host_address }}</p>
      <p v-if="result">
        <strong>{{ $gettext('known_hosts') }}:</strong> {{ result.known_hosts_path }}
      </p>
      <AAlert
        v-if="result?.persistence?.warning"
        type="warning"
        show-icon
        :message="result.persistence.warning"
      />
    </ACard>

    <div class="flex gap-2">
      <AButton type="primary" :loading="scanning" @click="scan(false)">
        {{ $gettext('Scan host keys') }}
      </AButton>
    </div>

    <AAlert
      v-if="scanError"
      type="error"
      show-icon
      :message="$gettext('Failed to scan host keys')"
      :description="scanError"
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
            <div class="flex items-center justify-between">
              <strong>{{ item.algorithm }}</strong>
              <ATag :color="statusColor(item.status)">
                {{ item.status }}
              </ATag>
            </div>
            <p class="mt-2">
              <strong>{{ $gettext('Fingerprint') }}:</strong> {{ item.fingerprint }}
            </p>
            <p v-if="item.existing_fingerprint">
              <strong>{{ $gettext('Existing fingerprint') }}:</strong> {{ item.existing_fingerprint }}
            </p>
            <ACollapse class="mt-2">
              <ACollapsePanel key="pub" :header="$gettext('Public key')">
                <CodeBlock :code="item.public_key" language="ssh" />
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

    <ACollapse v-if="result?.stale_keys?.length">
      <ACollapsePanel key="stale" :header="$gettext('Advanced cleanup')">
        <AList :data-source="result.stale_keys">
          <template #renderItem="{ item }">
            <AListItem>
              <div class="w-full">
                <ATag color="warning">
                  stale
                </ATag>
                <strong class="ml-2">{{ item.algorithm }}</strong>
                <div class="text-secondary text-sm mt-1">
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
      v-if="result && !hasChangedKey && hasOnlyTrustedKeys"
      type="success"
      show-icon
      :message="$gettext('Host identity is trusted. You may continue to verification.')"
    />
    <AAlert
      v-if="hasChangedKey"
      type="error"
      show-icon
      :message="$gettext('Resolve changed host keys before continuing.')"
    />
  </div>
</template>
