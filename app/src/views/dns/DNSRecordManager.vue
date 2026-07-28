<script setup lang="ts">
import type { DNSRecord, DNSRecordLine, RecordListParams, RecordPayload } from '@/api/dns'
import { PlusOutlined, ReloadOutlined } from '@ant-design/icons-vue'
import { message } from 'ant-design-vue'
import { computed, onBeforeUnmount, onMounted, ref } from 'vue'
import { useRoute, useRouter } from 'vue-router'
import { dnsApi } from '@/api/dns'
import FooterToolBar from '@/components/FooterToolbar'
import { useDnsStore } from '@/pinia/moudule/dns'
import DNSRecordFilter from '@/views/dns/components/DNSRecordFilter.vue'
import DNSRecordForm from '@/views/dns/components/DNSRecordForm.vue'
import DNSRecordTable from '@/views/dns/components/DNSRecordTable.vue'

interface DNSRecordTableInstance {
  resetPagination: () => void
}

const route = useRoute()
const store = useDnsStore()
const router = useRouter()
const recordTable = useTemplateRef<DNSRecordTableInstance>('recordTable')

const filters = ref<RecordListParams>({
  name: '',
  type: '',
})

const domainId = computed(() => Number(route.params.id))

const isDrawerOpen = ref(false)
const isSavingRecord = ref(false)
const editingRecord = ref<DNSRecord | null>(null)
const recordLines = ref<DNSRecordLine[]>([])
const isRecordLinesLoading = ref(false)
const formModel = ref<RecordPayload>({
  type: 'A',
  name: '@',
  content: '',
  ttl: 600,
})

const isCloudflare = computed(() => {
  const provider = store.currentDomain?.dns_credential?.provider ?? ''
  return provider.toLowerCase().includes('cloudflare')
})

const isAliDNS = computed(() => {
  return store.currentDomain?.dns_credential?.provider_code?.toLowerCase() === 'alidns'
})

const isHuaweiCloud = computed(() => {
  return store.currentDomain?.dns_credential?.provider_code?.toLowerCase() === 'huaweicloud'
})

const defaultRecordLine = computed(() => {
  if (isAliDNS.value)
    return 'default'
  if (isHuaweiCloud.value)
    return 'default_view'
  return undefined
})

const supportsRecordLines = computed(() => Boolean(defaultRecordLine.value))

const showProxiedToggle = computed(() => isCloudflare.value)

const showCommentField = computed(() => isCloudflare.value)

const contentSuggestions = computed(() => {
  const unique = new Set<string>()
  store.records.forEach(record => {
    const type = record.type?.toUpperCase?.() ?? ''
    if (record.content && (type === 'A' || type === 'CNAME')) {
      unique.add(record.content)
    }
  })
  return [...unique]
})

const pageTitle = computed(() => {
  return store.currentDomain?.domain ?? $gettext('DNS Records')
})

async function initData() {
  await store.fetchDomainDetail(domainId.value)
  await fetchRecords()
  if (supportsRecordLines.value) {
    await fetchRecordLines()
  }
}

async function fetchRecordLines() {
  isRecordLinesLoading.value = true
  try {
    const { data } = await dnsApi.listRecordLines(domainId.value)
    recordLines.value = data
  }
  finally {
    isRecordLinesLoading.value = false
  }
}

async function fetchRecords() {
  await store.fetchAllRecords(domainId.value, filters.value)
}

function openCreateDrawer() {
  editingRecord.value = null
  formModel.value = {
    type: 'A',
    name: '@',
    content: '',
    ttl: 600,
    line: defaultRecordLine.value,
  }
  isDrawerOpen.value = true
}

function openEditDrawer(record: DNSRecord) {
  editingRecord.value = record
  formModel.value = {
    type: record.type,
    name: record.name,
    content: record.content,
    ttl: record.ttl,
    line: record.line || defaultRecordLine.value,
    priority: record.priority,
    weight: record.weight,
    proxied: record.proxied,
    comment: record.comment,
  }
  isDrawerOpen.value = true
}

function closeRecordDrawer() {
  if (isSavingRecord.value)
    return

  isDrawerOpen.value = false
}

async function handleSubmit() {
  if (isSavingRecord.value)
    return

  isSavingRecord.value = true
  try {
    if (editingRecord.value) {
      await store.updateRecord(domainId.value, editingRecord.value.id, formModel.value)
      message.success($gettext('Record updated'))
    }
    else {
      await store.createRecord(domainId.value, formModel.value)
      message.success($gettext('Record created'))
    }
    await fetchRecords()
    isDrawerOpen.value = false
  }
  finally {
    isSavingRecord.value = false
  }
}

async function handleDelete(record: DNSRecord) {
  await store.deleteRecord(domainId.value, record.id)
  await fetchRecords()
  message.success($gettext('Record deleted'))
}

function handleFilterSubmit() {
  recordTable.value?.resetPagination()
  fetchRecords()
}

onMounted(() => {
  initData()
})

onBeforeUnmount(() => {
  store.resetRecords()
})
</script>

<template>
  <div class="record-manager">
    <ACard>
      <template #title>
        <ASpace align="center">
          {{ pageTitle }}
          <ATag v-if="store.currentDomain?.dns_credential?.provider">
            {{ store.currentDomain?.dns_credential?.provider }}
          </ATag>
        </ASpace>
      </template>
      <template #extra>
        <AButton type="link" size="small" @click="fetchRecords">
          <template #icon>
            <ReloadOutlined />
          </template>
          {{ $gettext('Refresh') }}
        </AButton>
        <AButton type="link" size="small" @click="openCreateDrawer">
          <template #icon>
            <PlusOutlined />
          </template>
          {{ $gettext('Add Record') }}
        </AButton>
      </template>

      <DNSRecordFilter v-model:filters="filters" @submit="handleFilterSubmit" />

      <DNSRecordTable
        ref="recordTable"
        class="mt-4"
        :records="store.records"
        :loading="store.recordsLoading"
        :show-proxied="showProxiedToggle"
        :show-comment="showCommentField"
        :show-line="supportsRecordLines"
        :line-options="recordLines"
        @edit="openEditDrawer"
        @delete="handleDelete"
      />
    </ACard>

    <ADrawer
      :open="isDrawerOpen"
      :title="editingRecord ? $gettext('Edit Record') : $gettext('Create Record')"
      width="480"
      @close="closeRecordDrawer"
    >
      <DNSRecordForm
        v-model:record="formModel"
        :show-name="true"
        :show-proxied="showProxiedToggle"
        :show-comment="showCommentField"
        :show-line="supportsRecordLines"
        :line-options="recordLines"
        :is-line-loading="isRecordLinesLoading"
        :default-line-code="defaultRecordLine"
        :line-disabled="Boolean(editingRecord) && isHuaweiCloud"
        :value-suggestions="contentSuggestions"
      />
      <template #footer>
        <ASpace>
          <AButton :disabled="isSavingRecord" @click="closeRecordDrawer">
            {{ $gettext('Cancel') }}
          </AButton>
          <AButton type="primary" :loading="isSavingRecord" @click="handleSubmit">
            {{ $gettext('Save') }}
          </AButton>
        </ASpace>
      </template>
    </ADrawer>

    <FooterToolBar>
      <AButton @click="router.push('/dns/domains')">
        {{ $gettext('Back') }}
      </AButton>
    </FooterToolBar>
  </div>
</template>

<style scoped lang="less">
.record-manager {
  padding-bottom: 24px;
}
</style>
