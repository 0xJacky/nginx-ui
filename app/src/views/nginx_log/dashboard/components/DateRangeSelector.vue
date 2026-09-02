<script setup lang="ts">
import type { MenuProps } from 'antdv-next'
import { DownOutlined, ReloadOutlined } from '@antdv-next/icons'
import { Card, DatePicker } from 'antdv-next'
import dayjs from 'dayjs'
import { computed } from 'vue'

defineProps<{
  logPath: string
  refreshLoading?: boolean
}>()

const emit = defineEmits<{
  refresh: []
}>()

const dateRange = defineModel<[dayjs.Dayjs, dayjs.Dayjs]>('dateRange', { required: true })

const { RangePicker } = DatePicker

// Date range presets for dashboard (daily-based)
const datePresets = [
  { key: 'last-7-days', label: () => $gettext('Last 7 days'), value: () => [dayjs().subtract(7, 'day').startOf('day'), dayjs().endOf('day')] as [dayjs.Dayjs, dayjs.Dayjs] },
  { key: 'last-14-days', label: () => $gettext('Last 14 days'), value: () => [dayjs().subtract(14, 'day').startOf('day'), dayjs().endOf('day')] as [dayjs.Dayjs, dayjs.Dayjs] },
  { key: 'last-30-days', label: () => $gettext('Last 30 days'), value: () => [dayjs().subtract(30, 'day').startOf('day'), dayjs().endOf('day')] as [dayjs.Dayjs, dayjs.Dayjs] },
  { key: 'last-90-days', label: () => $gettext('Last 90 days'), value: () => [dayjs().subtract(90, 'day').startOf('day'), dayjs().endOf('day')] as [dayjs.Dayjs, dayjs.Dayjs] },
  { key: 'this-month', label: () => $gettext('This month'), value: () => [dayjs().startOf('month'), dayjs().endOf('day')] as [dayjs.Dayjs, dayjs.Dayjs] },
  { key: 'last-month', label: () => $gettext('Last month'), value: () => [dayjs().subtract(1, 'month').startOf('month'), dayjs().subtract(1, 'month').endOf('month')] as [dayjs.Dayjs, dayjs.Dayjs] },
]

const datePresetMenuItems = computed<MenuProps['items']>(() => datePresets.map(preset => ({
  key: preset.key,
  label: preset.label(),
})))

// Apply date preset
function applyDatePreset(preset: { value: () => [dayjs.Dayjs, dayjs.Dayjs] }) {
  const range = preset.value()
  dateRange.value = range as [dayjs.Dayjs, dayjs.Dayjs]
}
</script>

<template>
  <Card size="small" class="mb-4">
    <div class="mb-2 text-sm font-medium text-gray-700 dark:text-gray-300">
      {{ $gettext('Date Range') }}
    </div>
    <ASpace wrap>
      <ADropdown placement="bottomLeft">
        <template #popupRender>
          <AMenu :items="datePresetMenuItems" @click="({ key }) => applyDatePreset(datePresets.find(preset => preset.key === key)!)" />
        </template>
        <AButton>
          {{ $gettext('Quick Select') }}
          <DownOutlined />
        </AButton>
      </ADropdown>
      <RangePicker
        v-model:value="dateRange"
        :placeholder="[$gettext('Start Date'), $gettext('End Date')]"
      />
      <AButton
        type="default"
        :loading="refreshLoading"
        @click="emit('refresh')"
      >
        <template #icon>
          <ReloadOutlined />
        </template>
      </AButton>
    </ASpace>
  </Card>
</template>
