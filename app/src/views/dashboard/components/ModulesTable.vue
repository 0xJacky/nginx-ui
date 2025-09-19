<script setup lang="tsx">
import type { TableColumnType } from 'ant-design-vue'
import type { NgxModule } from '@/api/ngx'
import { ReloadOutlined, SearchOutlined } from '@ant-design/icons-vue'
import { Button as AButton, Input as AInput, message } from 'ant-design-vue'
import ngx from '@/api/ngx'
import { useGlobalStore } from '@/pinia'

const globalStore = useGlobalStore()
const { modules, modulesMap } = storeToRefs(globalStore)

const keyword = ref('')

const isRefreshing = ref(false)

async function refreshModules() {
  try {
    isRefreshing.value = true
    const res = await ngx.refresh_modules()
    const list = res.modules || []
    modules.value = list
    modulesMap.value = list.reduce((acc, m) => {
      acc[m.name] = m
      return acc
    }, {} as Record<string, NgxModule>)
    message.success($gettext('Modules cache refreshed'))
  }
  catch (err) {
    console.error(err)
  }
  finally {
    isRefreshing.value = false
  }
}

const filteredModules = computed<NgxModule[]>(() => {
  const k = keyword.value.trim().toLowerCase()
  if (!k)
    return modules.value
  return modules.value.filter(m =>
    (m.name && m.name.toLowerCase().includes(k))
    || (m.params && m.params.toLowerCase().includes(k)),
  )
})

// Modules columns
const modulesColumns: TableColumnType[] = [
  {
    title: $gettext('Module'),
    dataIndex: 'name',
    width: '800px',
    customRender: args => {
      return (
        <div>
          <div>{args.record.name}</div>
          <div class="text-sm text-gray-500">{args.record.params}</div>
        </div>
      )
    },
  },
  {
    title: $gettext('Type'),
    dataIndex: 'dynamic',
    width: '100px',
    filters: [
      { text: $gettext('Dynamic'), value: true },
      { text: $gettext('Static'), value: false },
    ],
    onFilter: (value, record) => record.dynamic === value,
    customRender: ({ record }) => {
      return <span>{record.dynamic ? $gettext('Dynamic') : $gettext('Static')}</span>
    },
  },
  {
    title: $gettext('Status'),
    dataIndex: 'loaded',
    width: '100px',
    filters: [
      { text: $gettext('Loaded'), value: true },
      { text: $gettext('Not Loaded'), value: false },
    ],
    onFilter: (value, record) => record.loaded === value,
    customRender: ({ record }) => {
      return <span>{record.loaded ? $gettext('Loaded') : $gettext('Not Loaded')}</span>
    },
  },
]
</script>

<template>
  <div class="overflow-x-auto">
    <ATable
      :columns="modulesColumns"
      :data-source="filteredModules"
      :pagination="false"
      size="middle"
      :scroll="{ x: '100%' }"
    >
      <template #title>
        <div class="flex items-center justify-between gap-2">
          <AInput
            v-model:value="keyword"
            :placeholder="$gettext('Search modules')"
            style="max-width: 260px"
            allow-clear
          >
            <template #prefix>
              <SearchOutlined />
            </template>
          </AInput>
          <div>
            <AButton :loading="isRefreshing" @click="refreshModules">
              <ReloadOutlined />
              <span class="ml-1">{{ $gettext('Refresh Modules Cache') }}</span>
            </AButton>
          </div>
        </div>
      </template>
    </ATable>
  </div>
</template>
