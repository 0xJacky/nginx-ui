<script setup lang="ts">
import { StdCurd } from '@uozi-admin/curd'
import { message } from 'ant-design-vue'
import notification from '@/api/notification'
import { useUserStore } from '@/pinia'
import notificationColumns from '@/views/notification/notificationColumns'

const { unreadCount } = storeToRefs(useUserStore())

const curd = useTemplateRef('curd')
function clear() {
  notification.clear().then(() => {
    message.success($gettext('Cleared successfully'))
    curd.value?.refresh()
    unreadCount.value = 0
  })
}

watch(unreadCount, () => {
  curd.value?.refresh()
})
</script>

<template>
  <StdCurd
    ref="curd"
    :scroll-x="1000"
    :title="$gettext('Notification')"
    :columns="notificationColumns"
    :api="notification"
    disable-modify
    disable-add
    disable-export
    disable-trash
  >
    <template #beforeListActions>
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
