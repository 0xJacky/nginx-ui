<script setup lang="tsx">
import type { CustomRender } from '@/components/StdDesign/StdDataDisplay/StdTableTransformer'
import type { Column } from '@/components/StdDesign/types'
import nginxLog from '@/api/nginx_log'
import StdCurd from '@/components/StdDesign/StdDataDisplay/StdCurd.vue'
import { input, select } from '@/components/StdDesign/StdDataEntry'
import { Tag } from 'ant-design-vue'

const router = useRouter()
const stdCurdRef = ref()

const columns: Column[] = [
  {
    title: () => $gettext('Type'),
    dataIndex: 'type',
    customRender: (args: CustomRender) => {
      return args.record?.type === 'access' ? <Tag color="success">{ $gettext('Access Log') }</Tag> : <Tag color="orange">{ $gettext('Error Log') }</Tag>
    },
    sorter: true,
    search: {
      type: select,
      mask: {
        access: () => $gettext('Access Log'),
        error: () => $gettext('Error Log'),
      },
    },
    width: 200,
  },
  {
    title: () => $gettext('Name'),
    dataIndex: 'name',
    sorter: true,
    search: {
      type: input,
    },
  },
  {
    title: () => $gettext('Path'),
    dataIndex: 'path',
    sorter: true,
    search: {
      type: input,
    },
  },
  {
    title: () => $gettext('Action'),
    dataIndex: 'action',
  },
]

function viewLog(record: { type: string, path: string }) {
  router.push({
    path: `/nginx_log/${record.type}`,
    query: {
      log_path: record.path,
    },
  })
}
</script>

<template>
  <StdCurd
    ref="stdCurdRef"
    :title="$gettext('Log List')"
    :columns="columns"
    :api="nginxLog"
    disable-add
    disable-delete
    disable-view
    disable-modify
  >
    <template #actions="{ record }">
      <AButton type="link" size="small" @click="viewLog(record)">
        {{ $gettext('View') }}
      </AButton>
    </template>
  </StdCurd>
</template>

<style scoped lang="less">

</style>
