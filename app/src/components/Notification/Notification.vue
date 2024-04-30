<script setup lang="ts">
import { BellOutlined, CheckCircleOutlined, CloseCircleOutlined, DeleteOutlined, InfoCircleOutlined, WarningOutlined } from '@ant-design/icons-vue'
import type { Ref } from 'vue'
import { message } from 'ant-design-vue'
import notification from '@/api/notification'
import type { Notification } from '@/api/notification'
import { NotificationTypeT } from '@/constants'
import { useUserStore } from '@/pinia'

const loading = ref(false)

const { unreadCount } = storeToRefs(useUserStore())

const data = ref([]) as Ref<Notification[]>
function init() {
  loading.value = true
  notification.get_list().then(r => {
    data.value = r.data
    unreadCount.value = r.pagination.total
  }).catch(e => {
    message.error($gettext(e?.message ?? 'Server error'))
  }).finally(() => {
    loading.value = false
  })
}

onMounted(() => {
  init()
})

const open = ref(false)

watch(open, v => {
  if (v)
    init()
})

function clear() {
  notification.clear().then(() => {
    message.success($gettext('Cleared successfully'))
    data.value = []
    unreadCount.value = 0
  }).catch(e => {
    message.error($gettext(e?.message ?? 'Server error'))
  })
}

function remove(id: number) {
  notification.destroy(id).then(() => {
    message.success($gettext('Removed successfully'))
    init()
  }).catch(e => {
    message.error($gettext(e?.message ?? 'Server error'))
  })
}

const router = useRouter()
function viewAll() {
  router.push('/notifications')
  open.value = false
}
</script>

<template>
  <span class="cursor-pointer">
    <APopover
      v-model:open="open"
      placement="bottomRight"
      overlay-class-name="notification-popover"
      trigger="click"
    >
      <ABadge
        :count="unreadCount"
        dot
      >
        <BellOutlined />
      </ABadge>
      <template #content>
        <div class="flex justify-between items-center p-2">
          <h3 class="mb-0">{{ $gettext('Notifications') }}</h3>
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
        </div>

        <ADivider class="mt-2 mb-2" />

        <AList
          :data-source="data"
          class="max-h-96 overflow-scroll"
        >
          <template #renderItem="{ item }">
            <AListItem>
              <template #actions>
                <span
                  key="list-loadmore-remove"
                  class="cursor-pointer"
                  @click="remove(item.id)"
                >
                  <DeleteOutlined />
                </span>
              </template>
              <AListItemMeta
                :title="item.title"
                :description="item.details"
              >
                <template #avatar>
                  <div>
                    <CloseCircleOutlined
                      v-if="item.type === NotificationTypeT.Error"
                      class="text-red-500"
                    />
                    <WarningOutlined
                      v-else-if="item.type === NotificationTypeT.Warning"
                      class="text-orange-400"
                    />
                    <InfoCircleOutlined
                      v-else-if="item.type === NotificationTypeT.Info"
                      class="text-blue-500"
                    />
                    <CheckCircleOutlined
                      v-else-if="item.type === NotificationTypeT.Success"
                      class="text-green-500"
                    />
                  </div>
                </template>
              </AListItemMeta>
            </AListItem>
          </template>
        </AList>
        <ADivider class="m-0 mb-2" />
        <div class="flex justify-center p-2">
          <a @click="viewAll">{{ $gettext('View all notifications') }}</a>
        </div>
      </template>
    </APopover>
  </span>
</template>

<style lang="less">
.notification-popover {
  width: 400px;
}
</style>

<style scoped lang="less">
:deep(.ant-list-item-meta) {
  align-items: center !important;
}

:deep(.ant-list-item-meta-avatar) {
  font-size: 24px;
}
</style>
