<script setup lang="tsx">
import type { CustomRenderArgs, StdTableColumn } from '@uozi-admin/curd'
import type { DNSProvider } from '@/api/auto_cert'
import type { DnsCredential } from '@/api/dns_credential'
import { datetimeRender, StdCurd } from '@uozi-admin/curd'
import auto_cert from '@/api/auto_cert'
import dns_credential from '@/api/dns_credential'
import DNSChallenge from './components/DNSChallenge.vue'

const { message } = App.useApp()

const dnsProviders = ref<DNSProvider[]>([])
const refCurd = useTemplateRef('refCurd')

const editorVisible = ref(false)
const editorSaving = ref(false)
const editorData = ref<DnsCredential>({} as DnsCredential)

onMounted(async () => {
  dnsProviders.value = await auto_cert.get_dns_providers()
})

function ensureConfiguration(target: DnsCredential) {
  if (!target.configuration) {
    target.configuration = {
      credentials: {},
      additional: {},
    }
    return
  }

  target.configuration.credentials ??= {}
  target.configuration.additional ??= {}
}

async function openEditModal(record: DnsCredential) {
  try {
    const detail = await dns_credential.getItem(record.id)
    editorData.value = { ...detail }
  }
  catch {
    // Fall back to row data so users can still close and continue editing.
    editorData.value = { ...record }
  }

  ensureConfiguration(editorData.value)
  editorVisible.value = true
}

function closeEditorModal() {
  if (editorSaving.value)
    return

  editorVisible.value = false
}

async function saveEditorModal() {
  const name = editorData.value.name?.trim()
  if (!name) {
    message.error($gettext('Name cannot be empty'))
    return
  }

  if (!editorData.value.code) {
    message.error($gettext('Please select DNS provider'))
    return
  }

  ensureConfiguration(editorData.value)

  editorSaving.value = true
  try {
    const payload = {
      ...editorData.value,
      name,
    }
    await dns_credential.updateItem(editorData.value.id, payload)

    message.success($gettext('Save successfully'))
    editorVisible.value = false
    refCurd.value?.refresh()
  }
  // eslint-disable-next-line ts/no-explicit-any
  catch (error: any) {
    message.error(error?.message ?? $gettext('Server error'))
  }
  finally {
    editorSaving.value = false
  }
}

const columns: StdTableColumn[] = [{
  title: () => $gettext('Name'),
  dataIndex: 'name',
  sorter: true,
  pure: true,
  edit: {
    type: 'input',
  },
  search: true,
}, {
  title: () => $gettext('Provider'),
  dataIndex: 'provider_code',
  customRender: ({ record }: CustomRenderArgs) => {
    return record.provider
  },
  sorter: true,
  pure: true,
  search: {
    type: 'select',
    select: {
      remote: {
        valueKey: 'code',
        labelKey: 'name',
        api: async () => {
          return {
            data: await auto_cert.get_dns_providers(),
          }
        },
      },
      showSearch: true,
      filterOption: (input, option) => {
        return option?.label?.toLowerCase().includes(input.toLowerCase()) ?? false
      },
    },
  },
}, {
  title: () => $gettext('Configuration'),
  dataIndex: 'code',
  edit: {
    type: (context: { formData: DnsCredential }) => {
      return <DNSChallenge v-model:data={context.formData} />
    },
    formItem: {
      hiddenLabelInEdit: true,
    },
  },
  hiddenInTable: true,
  hiddenInDetail: true,
}, {
  title: () => $gettext('Updated at'),
  dataIndex: 'updated_at',
  customRender: datetimeRender,
  sorter: true,
  pure: true,
}, {
  title: () => $gettext('Actions'),
  dataIndex: 'actions',
  fixed: 'right',
}]
</script>

<template>
  <StdCurd
    ref="refCurd"
    :title="$gettext('DNS Credentials')"
    :api="dns_credential"
    :columns="columns"
    disable-router-query
    disable-export
    @edit-item="openEditModal"
  >
    <template #beforeForm>
      <AAlert
        class="mb-4"
        type="info"
        show-icon
        :title="$gettext('Note')"
      >
        <template #description>
          <p>
            {{ $gettext('Please fill in the API authentication credentials provided by your DNS provider.') }}
          </p>
          <p>
            {{ $gettext('Please note that the unit of time configurations below are all in seconds.') }}
          </p>
        </template>
      </AAlert>
    </template>
  </StdCurd>

  <AModal
    :open="editorVisible"
    :title="$gettext('Modify DNS Credential')"
    :closable="!editorSaving"
    :mask-closable="!editorSaving"
    :confirm-loading="editorSaving"
    :footer="null"
    width="680"
    @cancel="closeEditorModal"
  >
    <AAlert
      class="mb-4"
      type="info"
      show-icon
      :title="$gettext('Note')"
    >
      <template #description>
        <p>
          {{ $gettext('Please fill in the API authentication credentials provided by your DNS provider.') }}
        </p>
        <p>
          {{ $gettext('Please note that the unit of time configurations below are all in seconds.') }}
        </p>
      </template>
    </AAlert>

    <AForm layout="vertical">
      <AFormItem :label="$gettext('Name')" required>
        <AInput v-model:value="editorData.name" />
      </AFormItem>

      <AFormItem :label="$gettext('Configuration')">
        <DNSChallenge v-model:data="editorData" />
      </AFormItem>
    </AForm>

    <div class="flex justify-end gap-2">
      <AButton :disabled="editorSaving" @click="closeEditorModal">
        {{ $gettext('Close') }}
      </AButton>
      <AButton type="primary" :loading="editorSaving" @click="saveEditorModal">
        {{ $gettext('Save') }}
      </AButton>
    </div>
  </AModal>
</template>

<style lang="less" scoped>

</style>
