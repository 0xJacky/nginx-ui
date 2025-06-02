<script setup lang="tsx">
import type { CustomRenderArgs, StdTableColumn } from '@uozi-admin/curd'
import { StdCurd } from '@uozi-admin/curd'
import { Tag } from 'ant-design-vue'
import nginxLog from '@/api/nginx_log'

const router = useRouter()
const stdCurdRef = ref()

const columns: StdTableColumn[] = [
  {
    title: () => $gettext('Type'),
    dataIndex: 'type',
    customRender: (args: CustomRenderArgs) => {
      return args.record?.type === 'access' ? <Tag color="green">{ $gettext('Access Log') }</Tag> : <Tag color="orange">{ $gettext('Error Log') }</Tag>
    },
    sorter: true,
    search: {
      type: 'select',
      select: {
        options: [
          {
            label: () => $gettext('Access Log'),
            value: 'access',
          },
          {
            label: () => $gettext('Error Log'),
            value: 'error',
          },
        ],
      },
    },
    width: 200,
  },
  {
    title: () => $gettext('Name'),
    dataIndex: 'name',
    sorter: true,
    search: {
      type: 'input',
    },
  },
  {
    title: () => $gettext('Path'),
    dataIndex: 'path',
    sorter: true,
    search: {
      type: 'input',
    },
  },
  {
    title: () => $gettext('Actions'),
    dataIndex: 'actions',
    fixed: 'right',
    width: 200,
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
    disable-export
    disable-delete
    disable-view
    disable-edit
  >
    <template #beforeActions="{ record }">
      <AButton type="link" size="small" @click="viewLog(record)">
        {{ $gettext('View') }}
      </AButton>
    </template>
  </StdCurd>
</template>

<style scoped lang="less">

</style>
