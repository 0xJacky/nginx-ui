<script setup lang="ts">
import notification from '@/api/notification'
import StdCurd from '@/components/StdDesign/StdDataDisplay/StdCurd.vue'
import { useUserStore } from '@/pinia'
import notificationColumns from '@/views/notification/notificationColumns'
import { message } from 'ant-design-vue'

const { unreadCount } = storeToRefs(useUserStore())

const curd = useTemplateRef('curd')
function clear() {
  notification.clear().then(() => {
    message.success($gettext('Cleared successfully'))
    curd.value?.get_list()
    unreadCount.value = 0
  }).catch(e => {
    message.error($gettext(e?.message ?? 'Server error'))
  })
}

watch(unreadCount, () => {
  curd.value?.get_list()
})
</script>

<template>
  <StdCurd
    ref="curd"
    :title="$gettext('Notification')"
    :columns="notificationColumns"
    :api="notification"
    disable-modify
    disable-add
    disable-trash
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
