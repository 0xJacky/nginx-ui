<script setup lang="tsx">
import type { TableColumnType } from 'ant-design-vue'
import type { FilterResetProps } from 'ant-design-vue/es/table/interface'
import { SearchOutlined } from '@ant-design/icons-vue'
import { Button as AButton, Input as AInput } from 'ant-design-vue'
import { useGlobalStore } from '@/pinia'

const globalStore = useGlobalStore()
const { modules } = storeToRefs(globalStore)

const searchText = ref('')
const searchInput = ref<HTMLInputElement>()

function handleSearch(selectedKeys: string[], confirm: () => void) {
  confirm()
  searchText.value = selectedKeys[0]
}

function handleReset(clearFilters?: (param?: FilterResetProps) => void) {
  clearFilters?.({ confirm: true })
  searchText.value = ''
}

// Modules columns
const modulesColumns: TableColumnType[] = [
  {
    title: $gettext('Module'),
    dataIndex: 'name',
    width: '800px',
    filterDropdown: ({ setSelectedKeys, selectedKeys, confirm, clearFilters }) => (
      <div style="padding: 8px">
        <AInput
          ref={searchInput}
          value={selectedKeys[0]}
          placeholder={$gettext('Search module name')}
          style="width: 188px; margin-bottom: 8px; display: block;"
          onInput={e => setSelectedKeys(e.target.value ? [e.target.value] : [])}
          onPressEnter={() => handleSearch(selectedKeys as string[], confirm)}
          class="mb-2"
        />
        <div class="flex justify-between">
          <AButton
            type="primary"
            onClick={() => handleSearch(selectedKeys as string[], confirm)}
            size="small"
            class="mr-2"
          >
            {$gettext('Search')}
          </AButton>
          <AButton
            onClick={() => handleReset(clearFilters)}
            size="small"
          >
            {$gettext('Reset')}
          </AButton>
        </div>
      </div>
    ),
    filterIcon: filtered => (
      <SearchOutlined style={{ color: filtered ? '#1890ff' : undefined }} />
    ),
    onFilter: (value, record) =>
      record.name && record.name.toString().toLowerCase().includes((value as string).toLowerCase()),
    onFilterDropdownVisibleChange: visible => {
      if (visible) {
        setTimeout(() => {
          if (searchInput.value) {
            searchInput.value.focus()
          }
        }, 100)
      }
    },
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
      :data-source="modules"
      :pagination="false"
      size="middle"
      :scroll="{ x: '100%' }"
    />
  </div>
</template>
