<script setup lang="ts">
import type { DDNSDomainItem, DNSRecord, UpdateDDNSPayload } from '@/api/dns'
import { ReloadOutlined } from '@ant-design/icons-vue'
import { message } from 'ant-design-vue'
import dayjs from 'dayjs'
import { computed, onMounted, ref } from 'vue'
import { dnsApi } from '@/api/dns'
import { useDnsStore } from '@/pinia/moudule/dns'

const store = useDnsStore()

const loading = computed(() => store.ddnsListLoading)
const items = computed(() => store.ddnsList)

const drawerOpen = ref(false)
const saving = ref(false)
const currentDomain = ref<DDNSDomainItem | null>(null)
const ddnsForm = ref<UpdateDDNSPayload>({
  enabled: false,
  interval_seconds: 300,
  record_ids: [],
})

const records = ref<DNSRecord[]>([])
const recordsLoading = ref(false)

const recordOptions = computed(() => {
  const opts = new Map<string, { value: string, label: string }>()
  records.value
    .filter(item => {
      const type = item.type?.toUpperCase?.()
      return type === 'A' || type === 'AAAA'
    })
    .forEach(item => {
      opts.set(item.id, {
        value: item.id,
        label: `${item.name} (${item.type.toUpperCase?.() ?? ''})`,
      })
    })

  currentDomain.value?.config.targets?.forEach(target => {
    opts.set(target.id, {
      value: target.id,
      label: `${target.name} (${target.type})`,
    })
  })

  return Array.from(opts.values())
})

function filterRecordOption(input: string, option?: { label: string, value: string }) {
  if (!option)
    return false
  const keyword = input.toLowerCase()
  return option.label.toLowerCase().includes(keyword)
}

const columns = [
  {
    title: $gettext('Domain'),
    dataIndex: 'domain',
    key: 'domain',
  },
  {
    title: $gettext('Credential'),
    key: 'credential',
    customRender: ({ record }: { record: DDNSDomainItem }) => record.credential_name ?? '-',
  },
  {
    title: $gettext('Provider'),
    key: 'provider',
    customRender: ({ record }: { record: DDNSDomainItem }) => record.credential_provider ?? '-',
  },
  {
    title: $gettext('Status'),
    key: 'status',
  },
  {
    title: $gettext('Interval'),
    key: 'interval',
  },
  {
    title: $gettext('Targets'),
    key: 'targets',
  },
  {
    title: $gettext('Last run'),
    key: 'last',
  },
  {
    title: $gettext('Actions'),
    key: 'actions',
  },
]

async function init() {
  await store.fetchDDNSList()
}

function formatTime(value?: string) {
  if (!value)
    return $gettext('Not run yet')
  return dayjs(value).format('YYYY-MM-DD HH:mm:ss')
}

async function openDrawer(record: DDNSDomainItem) {
  currentDomain.value = record
  ddnsForm.value = {
    enabled: record.config.enabled,
    interval_seconds: record.config.interval_seconds,
    record_ids: record.config.targets?.map(t => t.id) ?? [],
  }
  drawerOpen.value = true
  await loadRecords(record.id)
}

async function loadRecords(domainId: number) {
  recordsLoading.value = true
  try {
    const res = await dnsApi.listRecords(domainId, { per_page: 200 })
    records.value = res.data
  }
  finally {
    recordsLoading.value = false
  }
}

async function saveDDNS() {
  if (!currentDomain.value)
    return
  saving.value = true
  try {
    await store.updateDDNSConfig(currentDomain.value.id, ddnsForm.value)
    await store.refreshDDNSItem(currentDomain.value.id)
    message.success($gettext('DDNS saved'))
    drawerOpen.value = false
  }
  finally {
    saving.value = false
  }
}

onMounted(() => {
  init()
})
</script>

<template>
  <div class="ddns-page">
    <ACard>
      <template #title>
        <ASpace align="center">
          {{ $gettext('DDNS Overview') }}
        </ASpace>
      </template>
      <template #extra>
        <AButton type="link" size="small" :loading="loading" @click="init">
          <template #icon>
            <ReloadOutlined />
          </template>
          {{ $gettext('Refresh') }}
        </AButton>
      </template>

      <ATable
        :loading="loading"
        :data-source="items"
        :columns="columns"
        row-key="id"
        :pagination="false"
      >
        <template #bodyCell="{ column, record }">
          <template v-if="column.key === 'status'">
            <ATag :color="record.config.enabled ? 'green' : 'default'">
              {{ record.config.enabled ? $gettext('Enabled') : $gettext('Disabled') }}
            </ATag>
            <div v-if="record.config.last_error" class="text-red-500 text-xs">
              {{ record.config.last_error }}
            </div>
          </template>
          <template v-else-if="column.key === 'interval'">
            {{ record.config.interval_seconds }}s
          </template>
          <template v-else-if="column.key === 'targets'">
            <ASpace wrap size="small">
              <ATag v-for="target in record.config.targets" :key="target.id">
                {{ target.name }} ({{ target.type }})
              </ATag>
              <span v-if="!record.config.targets?.length">-</span>
            </ASpace>
          </template>
          <template v-else-if="column.key === 'last'">
            <div>{{ formatTime(record.config.last_run_at) }}</div>
            <div class="text-xs text-gray-500">
              IPv4: {{ record.config.last_ipv4 || '-' }} | IPv6: {{ record.config.last_ipv6 || '-' }}
            </div>
          </template>
          <template v-else-if="column.key === 'actions'">
            <ASpace>
              <AButton size="small" type="link" @click="openDrawer(record as DDNSDomainItem)">
                {{ $gettext('Configure') }}
              </AButton>
            </ASpace>
          </template>
        </template>
      </ATable>
    </ACard>

    <ADrawer
      :open="drawerOpen"
      :title="currentDomain ? `${$gettext('Configure DDNS')} - ${currentDomain.domain}` : ''"
      width="520"
      @close="drawerOpen = false"
    >
      <ASkeleton v-if="recordsLoading" active />
      <template v-else>
        <AForm layout="vertical">
          <AFormItem :label="$gettext('Enable DDNS')">
            <ASwitch v-model:checked="ddnsForm.enabled" />
          </AFormItem>
          <AFormItem :label="$gettext('Records')">
            <ASelect
              v-model:value="ddnsForm.record_ids"
              mode="tags"
              show-search
              :filter-option="(filterRecordOption as any)"
              :options="recordOptions"
              :placeholder="$gettext('Type or select A/AAAA records')"
              :disabled="!ddnsForm.enabled"
            />
          </AFormItem>
          <AFormItem :label="$gettext('Interval (seconds)')">
            <AInputNumber
              v-model:value="ddnsForm.interval_seconds"
              :min="60"
              :step="60"
              :disabled="!ddnsForm.enabled"
              style="width: 200px"
            />
          </AFormItem>
        </AForm>
        <div class="flex gap-2 mt-4">
          <AButton @click="drawerOpen = false">
            {{ $gettext('Cancel') }}
          </AButton>
          <AButton type="primary" :loading="saving" @click="saveDDNS">
            {{ $gettext('Save') }}
          </AButton>
        </div>
      </template>
    </ADrawer>
  </div>
</template>

<style scoped>
.ddns-page {
  padding-bottom: 16px;
}
</style>
