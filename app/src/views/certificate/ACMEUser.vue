<script setup lang="tsx">
import { Tag, message } from 'ant-design-vue'
import type { Column } from '@/components/StdDesign/types'
import { StdCurd } from '@/components/StdDesign/StdDataDisplay'
import type { AcmeUser } from '@/api/acme_user'
import acme_user from '@/api/acme_user'
import { input } from '@/components/StdDesign/StdDataEntry'
import type { customRender } from '@/components/StdDesign/StdDataDisplay/StdTableTransformer'
import { datetime } from '@/components/StdDesign/StdDataDisplay/StdTableTransformer'

const columns: Column[] = [
  {
    title: () => $gettext('Name'),
    dataIndex: 'name',
    sortable: true,
    pithy: true,
    edit: {
      type: input,
    },
  }, {
    title: () => $gettext('Email'),
    dataIndex: 'email',
    sortable: true,
    pithy: true,
    edit: {
      type: input,
    },
  }, {
    title: () => $gettext('CA Dir'),
    dataIndex: 'ca_dir',
    sortable: true,
    pithy: true,
    edit: {
      type: input,
      config: {
        placeholder() {
          return $gettext('If left blank, the default CA Dir will be used.')
        },
      },
    },
  }, {
    title: () => $gettext('Status'),
    dataIndex: ['registration', 'body', 'status'],
    customRender: (args: customRender) => {
      if (args.text === 'valid')
        return <Tag color="green">{$gettext('Valid')}</Tag>

      return <Tag color="red">{$gettext('Invalid')}</Tag>
    },
    sortable: true,
    pithy: true,
  }, {
    title: () => $gettext('Updated at'),
    dataIndex: 'updated_at',
    customRender: datetime,
    sortable: true,
    pithy: true,
  }, {
    title: () => $gettext('Action'),
    dataIndex: 'action',
  },
]

function register(id: number, data: AcmeUser) {
  acme_user.register(id).then(r => {
    data.registration = r.registration
    message.success($gettext('Register successfully'))
  }).catch(e => {
    message.error(e?.message ?? $gettext('Register failed'))
  })
}
</script>

<template>
  <StdCurd
    :title="$gettext('ACME User')"
    :columns="columns"
    :api="acme_user"
  >
    <template #edit="{ data }: {data: AcmeUser}">
      <template v-if="data.id > 0 ">
        <div class="mb-2">
          <label>{{ $gettext('Registration Status') }}</label>
        </div>
        <template v-if="data?.registration?.body?.status === 'valid'">
          <ATag color="green">
            {{ $gettext('Valid') }}
          </ATag>
        </template>
        <template v-else>
          <ATag color="red">
            {{ $gettext('Invalid') }}
          </ATag>
        </template>

        <AButton
          type="link"
          @click="register(data.id, data)"
        >
          {{ $gettext('Register') }}
        </AButton>
      </template>
    </template>
  </StdCurd>
</template>

<style scoped lang="less">

</style>
