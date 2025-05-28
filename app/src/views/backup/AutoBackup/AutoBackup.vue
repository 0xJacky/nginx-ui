<script setup lang="tsx">
import type { CustomRenderArgs, StdTableColumn } from '@uozi-admin/curd'
import type { AutoBackup } from '@/api/backup'
import { datetimeRender, StdCurd } from '@uozi-admin/curd'
import { FormItem, Input, Tag } from 'ant-design-vue'
import { autoBackup } from '@/api/backup'
import { CronEditor, StorageConfigEditor } from './components'

const columns: StdTableColumn[] = [
  {
    title: () => $gettext('Name'),
    dataIndex: 'name',
    sorter: true,
    pure: true,
    edit: {
      type: 'input',
      formItem: {
        required: true,
      },
    },
    search: true,
  },
  {
    title: () => $gettext('Backup Type'),
    dataIndex: 'backup_type',
    customRender: ({ text }: CustomRenderArgs) => {
      const typeMap = {
        nginx_and_nginx_ui: $gettext('Nginx and Nginx UI Config'),
        custom_dir: $gettext('Custom Directory'),
      }
      return typeMap[text as keyof typeof typeMap] || text
    },
    edit: {
      type: 'select',
      formItem: {
        required: true,
      },
      select: {
        options: [
          { label: $gettext('Nginx and Nginx UI Config'), value: 'nginx_and_nginx_ui' },
          { label: $gettext('Custom Directory'), value: 'custom_dir' },
        ],
      },
    },
    search: true,
    sorter: true,
    pure: true,
  },
  {
    title: () => $gettext('Backup Path'),
    dataIndex: 'backup_path',
    edit: {
      type: (formData: AutoBackup) => {
        if (formData.backup_type !== 'custom_dir')
          return <div />

        return (
          <FormItem class="mb-0" required={true} label={$gettext('Backup Path')}>
            <Input v-model:value={formData.backup_path} />
          </FormItem>
        )
      },
      formItem: {
        hiddenLabelInEdit: true,
      },
    },
    hiddenInTable: true,
  },
  {
    title: () => $gettext('Storage Type'),
    dataIndex: 'storage_type',
    customRender: ({ text }: CustomRenderArgs) => {
      const typeMap = {
        local: $gettext('Local'),
        s3: $gettext('S3'),
      }
      return typeMap[text as keyof typeof typeMap] || text
    },
    search: {
      type: 'select',
      select: {
        options: [
          { label: $gettext('Local'), value: 'local' },
          { label: $gettext('S3'), value: 's3' },
        ],
      },
    },
    sorter: true,
    pure: true,
  },
  {
    title: () => $gettext('Storage Configuration'),
    dataIndex: 'storage_type',
    edit: {
      type: (formData: AutoBackup) => {
        if (!formData.storage_type) {
          formData.storage_type = 'local'
        }
        return (
          <div>
            <div class="font-500 mb-4">{$gettext('Storage Configuration')}</div>
            <StorageConfigEditor v-model={formData} />
          </div>
        )
      },
      formItem: {
        hiddenLabelInEdit: true,
      },
    },
    hiddenInTable: true,
  },
  {
    title: () => $gettext('Schedule'),
    dataIndex: 'cron_expression',
    customRender: ({ text }: CustomRenderArgs) => {
      if (!text)
        return ''

      // Parse and display human-readable format
      const parts = text.trim().split(/\s+/)
      if (parts.length !== 5)
        return text

      const [minute, hour, dayOfMonth, month, dayOfWeek] = parts
      const timeStr = `${hour.padStart(2, '0')}:${minute.padStart(2, '0')}`

      if (dayOfMonth === '*' && month === '*' && dayOfWeek === '*') {
        return $gettext('Daily at %{time}', { time: timeStr })
      }

      if (dayOfMonth === '*' && month === '*' && dayOfWeek !== '*') {
        const weekDays = ['Sunday', 'Monday', 'Tuesday', 'Wednesday', 'Thursday', 'Friday', 'Saturday']
        const dayName = weekDays[Number.parseInt(dayOfWeek)] || 'Sunday'
        return $gettext('Weekly on %{day} at %{time}', { day: $gettext(dayName), time: timeStr })
      }

      if (dayOfMonth !== '*' && month === '*' && dayOfWeek === '*') {
        return $gettext('Monthly on day %{day} at %{time}', { day: dayOfMonth, time: timeStr })
      }

      return text
    },
    edit: {
      type: (formData: AutoBackup) => {
        if (!formData.cron_expression) {
          formData.cron_expression = '0 0 * * *'
        }
        return (
          <CronEditor v-model={formData.cron_expression} />
        )
      },
      formItem: {
        hiddenLabelInEdit: true,
      },
    },
    sorter: true,
    pure: true,
  },
  {
    title: () => $gettext('Status'),
    dataIndex: 'enabled',
    customRender: ({ text }: CustomRenderArgs) => {
      return text
        ? <Tag color="green">{$gettext('Enabled')}</Tag>
        : <Tag color="red">{$gettext('Disabled')}</Tag>
    },
    edit: {
      type: 'switch',
    },
    search: {
      type: 'select',
      select: {
        options: [
          { label: $gettext('Enabled'), value: 1 },
          { label: $gettext('Disabled'), value: 0 },
        ],
      },
    },
    sorter: true,
    pure: true,
  },
  {
    title: () => $gettext('Last Backup Time'),
    dataIndex: 'last_backup_time',
    customRender: datetimeRender,
    sorter: true,
    pure: true,
  },
  {
    title: () => $gettext('Last Backup Status'),
    dataIndex: 'last_backup_status',
    customRender: ({ text, record }: CustomRenderArgs) => {
      const statusMap = {
        pending: { color: 'orange', text: $gettext('Pending') },
        success: { color: 'green', text: $gettext('Success') },
        failed: { color: 'red', text: $gettext('Failed') },
      }
      const status = statusMap[text as keyof typeof statusMap]
      const statusTag = status ? <Tag color={status.color}>{status.text}</Tag> : text

      // Show error message below failed status
      if (text === 'failed' && record.last_backup_error) {
        return (
          <div>
            {statusTag}
            <div class="text-red-500 text-xs mt-1 max-w-xs break-words">
              {record.last_backup_error}
            </div>
          </div>
        )
      }

      return statusTag
    },
    search: {
      type: 'select',
      select: {
        options: [
          { label: $gettext('Pending'), value: 'pending' },
          { label: $gettext('Success'), value: 'success' },
          { label: $gettext('Failed'), value: 'failed' },
        ],
      },
    },
    sorter: true,
    pure: true,
  },
  {
    title: () => $gettext('Created at'),
    dataIndex: 'created_at',
    customRender: datetimeRender,
    sorter: true,
    pure: true,
  },
  {
    title: () => $gettext('Updated at'),
    dataIndex: 'updated_at',
    customRender: datetimeRender,
    sorter: true,
    pure: true,
  },
  {
    title: () => $gettext('S3 Endpoint'),
    dataIndex: 's3_endpoint',
    hiddenInTable: true,
    hiddenInEdit: true,
  },
  {
    title: () => $gettext('S3 Access Key ID'),
    dataIndex: 's3_access_key_id',
    hiddenInTable: true,
    hiddenInEdit: true,
  },
  {
    title: () => $gettext('S3 Secret Access Key'),
    dataIndex: 's3_secret_access_key',
    hiddenInTable: true,
    hiddenInEdit: true,
  },
  {
    title: () => $gettext('S3 Bucket'),
    dataIndex: 's3_bucket',
    hiddenInTable: true,
    hiddenInEdit: true,
  },
  {
    title: () => $gettext('S3 Region'),
    dataIndex: 's3_region',
    hiddenInTable: true,
    hiddenInEdit: true,
  },
  {
    title: () => $gettext('Actions'),
    dataIndex: 'actions',
    fixed: 'right',
  },
]
</script>

<template>
  <StdCurd
    :title="$gettext('Auto Backup')"
    :columns="columns"
    :api="autoBackup"
    disable-export
  />
</template>

<style lang="less">

</style>
