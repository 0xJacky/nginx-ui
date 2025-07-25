<script setup lang="ts">
import type { Ref } from 'vue'
import type { Notification } from '@/api/notification'
import { BellOutlined, CheckCircleOutlined, CloseCircleOutlined, DeleteOutlined, InfoCircleOutlined, WarningOutlined } from '@ant-design/icons-vue'
import { message, notification } from 'ant-design-vue'
import dayjs from 'dayjs'
import relativeTime from 'dayjs/plugin/relativeTime'
import notificationApi from '@/api/notification'
import { detailRender } from '@/components/Notification/detailRender'
import { useWebSocketEventBus } from '@/composables/useWebSocketEventBus'
import { NotificationTypeT } from '@/constants'
import { useUserStore } from '@/pinia'
import notifications from './notifications'

defineProps<{
  headerRef: HTMLElement
}>()

dayjs.extend(relativeTime)

const loading = ref(false)

const { unreadCount } = storeToRefs(useUserStore())

const data = ref([]) as Ref<Notification[]>

const { subscribe } = useWebSocketEventBus()

onMounted(() => {
  subscribe('notification', (data: Notification) => {
    const typeTrans = {
      0: 'error',
      1: 'warning',
      2: 'info',
      3: 'success',
    }

    notification[typeTrans[data.type]]({
      message: $gettext(data.title),
      description: detailRender({ text: data.details, record: data }),
    })
  })
})

function init() {
  loading.value = true
  notificationApi.getList({ sort: 'desc', order_by: 'created_at' }).then(r => {
    data.value = r.data
    unreadCount.value = r.pagination?.total || 0
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
  notificationApi.clear().then(() => {
    message.success($gettext('Cleared successfully'))
    data.value = []
    unreadCount.value = 0
    open.value = false
  })
}

function remove(id: number) {
  notificationApi.deleteItem(id).then(() => {
    message.success($gettext('Removed successfully'))
    init()
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
      :get-popup-container="() => headerRef"
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
              <AListItemMeta>
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
                <template #title>
                  <div class="flex justify-between items-center">
                    {{ $gettext(item.title) }}
                    <span class="text-xs text-trueGray-400 font-normal">
                      {{ dayjs(item.created_at).fromNow() }}
                    </span>
                  </div>
                </template>
                <template #description>
                  <div class="flex justify-between items-center">
                    <div>
                      {{ notifications[item.title]?.content(item.details)
                        || item.content || item.details }}
                    </div>
                    <span
                      key="list-loadmore-remove"
                      class="cursor-pointer"
                      @click="remove(item.id)"
                    >
                      <DeleteOutlined />
                    </span>
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
