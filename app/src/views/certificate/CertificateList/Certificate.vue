<script setup lang="tsx">
import type { DiscoveredCertificatePair } from '@/api/cert'
import { CloudUploadOutlined, SafetyCertificateOutlined } from '@ant-design/icons-vue'
import { StdTable } from '@uozi-admin/curd'
import { Tag } from 'ant-design-vue'
import cert from '@/api/cert'
import settings from '@/api/settings'
import { useGlobalStore } from '@/pinia'
import WildcardCertificate from '../components/DNSIssueCertificate.vue'
import RemoveCert from '../components/RemoveCert.vue'
import RetryCert from '../components/RetryCert.vue'
import certColumns from './certColumns'

const refWildcard = ref()
const refTable = ref()

const globalStore = useGlobalStore()

const { processingStatus } = storeToRefs(globalStore)

const discoveryVisible = ref(false)
const discoveryLoading = ref(false)
const discoveryImporting = ref(false)
const discoveryCandidates = ref<DiscoveredCertificatePair[]>([])
const selectedDiscoveryKeys = ref<string[]>([])
const discoveryPatternsConfigured = ref(true)
const { message } = App.useApp()

function discoveryRowKey(record: DiscoveredCertificatePair) {
  return record.fingerprint || `${record.ssl_certificate_path}|${record.ssl_certificate_key_path}`
}

const discoveryRowSelection = computed(() => ({
  selectedRowKeys: selectedDiscoveryKeys.value,
  onChange: (keys: (string | number)[]) => {
    selectedDiscoveryKeys.value = keys.map(String)
  },
}))

const discoveryColumns = computed(() => [
  {
    title: $gettext('Name'),
    dataIndex: 'name',
  },
  {
    title: $gettext('Type'),
    customRender: () => (
      <Tag bordered={false} color="purple">
        {$gettext('General Certificate')}
      </Tag>
    ),
  },
  {
    title: $gettext('SSL Certificate Path'),
    dataIndex: 'ssl_certificate_path',
    ellipsis: true,
  },
  {
    title: $gettext('SSL Certificate Key Path'),
    dataIndex: 'ssl_certificate_key_path',
    ellipsis: true,
  },
  {
    title: $gettext('Not After'),
    customRender: ({ record }: { record: DiscoveredCertificatePair }) => {
      return record.certificate_info?.not_after ?? '-'
    },
  },
])

async function scanConfiguredDiscovery() {
  discoveryLoading.value = true
  try {
    const currentSettings = await settings.get()
    discoveryPatternsConfigured.value = currentSettings.cert.discovery_patterns?.some(pattern => pattern.trim()) ?? false
    if (!discoveryPatternsConfigured.value) {
      discoveryCandidates.value = []
      selectedDiscoveryKeys.value = []
      return
    }

    const result = await cert.discover_new({
      configured: true,
      new_only: true,
    })
    discoveryCandidates.value = result.candidates ?? []
    selectedDiscoveryKeys.value = discoveryCandidates.value.map(discoveryRowKey)
  }
  catch (error) {
    console.error(error)
    message.error($gettext('Failed to scan certificates'))
  }
  finally {
    discoveryLoading.value = false
  }
}

async function openDiscovery() {
  discoveryVisible.value = true
  await scanConfiguredDiscovery()
}

async function importSelectedDiscoveredCerts() {
  const selected = new Set(selectedDiscoveryKeys.value)
  const candidates = discoveryCandidates.value.filter(item => selected.has(discoveryRowKey(item)))
  if (!candidates.length) {
    message.warning($gettext('Please select at least one certificate'))
    return
  }

  discoveryImporting.value = true
  try {
    for (const item of candidates) {
      await cert.import_existing({
        name: item.name,
        ssl_certificate_path: item.ssl_certificate_path,
        ssl_certificate_key_path: item.ssl_certificate_key_path,
        key_type: item.key_type,
      })
    }
    message.success($gettext('Import successfully'))
    discoveryVisible.value = false
    refTable.value?.refresh?.()
  }
  catch (error) {
    console.error(error)
    message.error($gettext('Failed to import certificate'))
  }
  finally {
    discoveryImporting.value = false
  }
}
</script>

<template>
  <ACard :title="$gettext('Certificates')">
    <template #extra>
      <AButton
        type="link"
        size="small"
        @click="openDiscovery"
      >
        <CloudUploadOutlined />
        {{ $gettext('Discover') }}
      </AButton>

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
    <AModal
      v-model:open="discoveryVisible"
      :title="$gettext('Discover Certificates')"
      :ok-text="$gettext('Import selected')"
      :confirm-loading="discoveryImporting"
      :ok-button-props="{ disabled: selectedDiscoveryKeys.length === 0 }"
      width="900px"
      @ok="importSelectedDiscoveredCerts"
    >
      <div class="mb-4 flex justify-end">
        <AButton
          :loading="discoveryLoading"
          @click="scanConfiguredDiscovery"
        >
          {{ $gettext('Scan') }}
        </AButton>
      </div>
      <AAlert
        v-if="!discoveryPatternsConfigured"
        type="info"
        show-icon
      >
        <template #message>
          {{ $gettext('No certificate discovery patterns configured') }}
        </template>
        <template #description>
          <span>
            {{ $gettext('Configure discovery patterns in certificate settings before scanning for new certificates.') }}
            <RouterLink :to="{ path: '/preference', query: { tab: 'cert' } }">
              {{ $gettext('Open certificate settings') }}
            </RouterLink>
          </span>
        </template>
      </AAlert>
      <ATable
        v-else
        :columns="discoveryColumns"
        :data-source="discoveryCandidates"
        :loading="discoveryLoading"
        :row-key="discoveryRowKey"
        :row-selection="discoveryRowSelection"
        :pagination="false"
        size="small"
      />
    </AModal>
  </ACard>
</template>

<style lang="less" scoped>

</style>
