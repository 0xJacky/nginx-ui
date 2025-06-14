<script setup lang="tsx">
import type { CustomRenderArgs, StdTableColumn } from '@uozi-admin/curd'
import type { DnsCredential } from '@/api/dns_credential'
import { datetimeRender, StdCurd } from '@uozi-admin/curd'
import dns_credential from '@/api/dns_credential'
import DNSChallenge from './components/DNSChallenge.vue'

const columns: StdTableColumn[] = [{
  title: () => $gettext('Name'),
  dataIndex: 'name',
  sorter: true,
  pure: true,
  edit: {
    type: 'input',
  },
}, {
  title: () => $gettext('Provider'),
  dataIndex: ['config', 'name'],
  customRender: ({ record }: CustomRenderArgs) => {
    return record.provider
  },
  sorter: true,
  pure: true,
}, {
  title: () => $gettext('Configuration'),
  dataIndex: 'code',
  edit: {
    type: (formData: DnsCredential) => {
      return <DNSChallenge v-model:data={formData} />
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
    :title="$gettext('DNS Credentials')"
    :api="dns_credential"
    :columns="columns"
    disable-export
  >
    <template #beforeForm>
      <AAlert
        class="mb-4"
        type="info"
        show-icon
        :message="$gettext('Note')"
      >
        <template #description>
          <p>
            {{ $gettext('Please fill in the API authentication credentials provided by your DNS provider.') }}
          </p>
          <p>
            {{ $gettext('We will add one or more TXT records to the DNS records of your domain for ownership verification.') }}
          </p>
          <p>
            {{ $gettext('Once the verification is complete, the records will be removed.') }}
          </p>
          <p>
            {{ $gettext('Please note that the unit of time configurations below are all in seconds.') }}
          </p>
        </template>
      </AAlert>
    </template>
  </StdCurd>
</template>

<style lang="less" scoped>

</style>
