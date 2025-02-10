<script setup lang="tsx">
import type { CustomRender } from '@/components/StdDesign/StdDataDisplay/StdTableTransformer'
import type { Column } from '@/components/StdDesign/types'
import dns_credential from '@/api/dns_credential'
import StdCurd from '@/components/StdDesign/StdDataDisplay/StdCurd.vue'
import { datetime } from '@/components/StdDesign/StdDataDisplay/StdTableTransformer'
import { input } from '@/components/StdDesign/StdDataEntry'
import DNSChallenge from './DNSChallenge.vue'

const columns: Column[] = [{
  title: () => $gettext('Name'),
  dataIndex: 'name',
  sorter: true,
  pithy: true,
  edit: {
    type: input,
  },
}, {
  title: () => $gettext('Provider'),
  dataIndex: ['config', 'name'],
  customRender: (args: CustomRender) => {
    return args.record.provider
  },
  sorter: true,
  pithy: true,
}, {
  title: () => $gettext('Updated at'),
  dataIndex: 'updated_at',
  customRender: datetime,
  sorter: true,
  pithy: true,
}, {
  title: () => $gettext('Action'),
  dataIndex: 'action',
}]
</script>

<template>
  <StdCurd
    :title="$gettext('DNS Credentials')"
    :api="dns_credential"
    :columns="columns"
  >
    <template #beforeEdit>
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
    <template #edit>
      <DNSChallenge />
    </template>
  </StdCurd>
</template>

<style lang="less" scoped>

</style>
