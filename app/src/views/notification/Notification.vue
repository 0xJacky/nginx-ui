<script setup lang="ts">
import { message } from 'ant-design-vue'
import StdCurd from '@/components/StdDesign/StdDataDisplay/StdCurd.vue'
import notification from '@/api/notification'
import type { Column } from '@/components/StdDesign/types'
import type { customRender } from '@/components/StdDesign/StdDataDisplay/StdTableTransformer'
import { datetime, mask } from '@/components/StdDesign/StdDataDisplay/StdTableTransformer'
import { NotificationType } from '@/constants'
import { useUserStore } from '@/pinia'
import { detailRender } from '@/components/Notification/detailRender'

const columns: Column[] = [{
  title: () => $gettext('Type'),
  dataIndex: 'type',
  customRender: mask(NotificationType),
  sortable: true,
  pithy: true,
}, {
  title: () => $gettext('Title'),
  dataIndex: 'title',
  customRender: (args: customRender) => {
    return h('span', $gettext(args.text))
  },
  pithy: true,
}, {
  title: () => $gettext('Details'),
  dataIndex: 'details',
  customRender: detailRender,
  pithy: true,
}, {
  title: () => $gettext('Created at'),
  dataIndex: 'created_at',
  sortable: true,
  customRender: datetime,
  pithy: true,
}, {
  title: () => $gettext('Action'),
  dataIndex: 'action',
}]

const { unreadCount } = storeToRefs(useUserStore())

const curd = ref()
function clear() {
  notification.clear().then(() => {
    message.success($gettext('Cleared successfully'))
    curd.value.get_list()
    unreadCount.value = 0
  }).catch(e => {
    message.error($gettext(e?.message ?? 'Server error'))
  })
}

watch(unreadCount, () => {
  curd.value.get_list()
})
</script>

<template>
  <StdCurd
    ref="curd"
    :title="$gettext('Notification')"
    :columns="columns"
    :api="notification"
    disabled-modify
    disable-add
  >
    <template #extra>
      <APopconfirm
        :cancel-text="$gettext('No')"
        :ok-text="$gettext('OK')"
        :title="$gettext('Are you sure you want to clear all notifications?')"
        placement="bottomRight"
        @confirm="clear"
      >
        <a>
          {{ $gettext('Clear') }}
        </a>
      </APopconfirm>
    </template>
  </StdCurd>
</template>

<style scoped lang="less">

</style>
