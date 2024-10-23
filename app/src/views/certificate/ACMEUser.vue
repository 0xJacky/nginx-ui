<script setup lang="tsx">
import { Tag, message } from 'ant-design-vue'
import type { Column } from '@/components/StdDesign/types'
import { StdCurd } from '@/components/StdDesign/StdDataDisplay'
import type { AcmeUser } from '@/api/acme_user'
import acme_user from '@/api/acme_user'
import { input, switcher } from '@/components/StdDesign/StdDataEntry'
import type { customRender } from '@/components/StdDesign/StdDataDisplay/StdTableTransformer'
import { datetime } from '@/components/StdDesign/StdDataDisplay/StdTableTransformer'

const columns: Column[] = [
  {
    title: () => $gettext('Name'),
    dataIndex: 'name',
    sorter: true,
    pithy: true,
    edit: {
      type: input,
      config: {
        required: true,
      },
    },
  }, {
    title: () => $gettext('Email'),
    dataIndex: 'email',
    sorter: true,
    pithy: true,
    edit: {
      type: input,
      config: {
        required: true,
      },
    },
  }, {
    title: () => $gettext('CA Dir'),
    dataIndex: 'ca_dir',
    sorter: true,
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
    title: () => $gettext('Proxy'),
    dataIndex: 'proxy',
    hiddenInTable: true,
    edit: {
      type: input,
      hint: $gettext('Register a user or use this account to issue a certificate through an HTTP proxy.'),
      config: {
        placeholder() {
          return $gettext('Leave blank if you don\'t need this.')
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
    sorter: true,
    pithy: true,
  }, {
    title: () => $gettext('Register On Startup'),
    dataIndex: 'register_on_startup',
    hiddenInTable: true,
    hiddenInDetail: true,
    edit: {
      type: switcher,
      hint: $gettext('When Enabled, Nginx UI will automatically re-register users upon startup. '
          + 'Generally, do not enable this unless you are in a dev environment and using Pebble as CA.'),
    },
  }, {
    title: () => $gettext('Updated at'),
    dataIndex: 'updated_at',
    customRender: datetime,
    sorter: true,
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
