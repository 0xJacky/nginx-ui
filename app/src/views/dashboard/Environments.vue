<script setup lang="ts">
import { useGettext } from 'vue3-gettext'
import Icon, { LinkOutlined, SendOutlined, ThunderboltOutlined } from '@ant-design/icons-vue'
import type ReconnectingWebSocket from 'reconnecting-websocket'
import type { Ref } from 'vue'
import { useSettingsStore } from '@/pinia'
import type { Node } from '@/api/environment'
import environment from '@/api/environment'
import logo from '@/assets/img/logo.png'
import pulse from '@/assets/svg/pulse.svg'
import { formatDateTime } from '@/lib/helper'
import NodeAnalyticItem from '@/views/dashboard/components/NodeAnalyticItem.vue'
import analytic from '@/api/analytic'

const { $gettext } = useGettext()

const data = ref([]) as Ref<Node[]>

const node_map = computed(() => {
  const o = {} as Record<number, Node>

  data.value.forEach(v => {
    o[v.id] = v
  })

  return o
})

let websocket: ReconnectingWebSocket | WebSocket

onMounted(() => {
  environment.get_list().then(r => {
    data.value = r.data
  })
  websocket = analytic.nodes()
  websocket.onmessage = async m => {
    const nodes = JSON.parse(m.data)

    Object.keys(nodes).forEach((v: string) => {
      const key = Number.parseInt(v)

      // update node online status
      if (node_map.value[key]) {
        Object.assign(node_map.value[key], nodes[key])
        node_map.value[key].response_at = new Date()
      }
    })
  }
})

onUnmounted(() => {
  websocket.close()
})

const { environment: env } = useSettingsStore()

function link_start(node: Node) {
  env.id = node.id
  env.name = node.name
}

const visible = computed(() => {
  if (env.id > 0)
    return false
  else
    return data.value?.length
})
</script>

<template>
  <ACard
    v-if="visible"
    class="env-list-card"
    :title="$gettext('Environments')"
  >
    <AList
      item-layout="horizontal"
      :data-source="data"
    >
      <template #renderItem="{ item }">
        <AListItem>
          <template #actions>
            <AButton
              type="primary"
              :disabled="env.id === item.id"
              ghost
              @click="link_start(item)"
            >
              <SendOutlined />
              {{ env.id !== item.id ? $gettext('Link Start') : $gettext('Connected') }}
            </AButton>
          </template>
          <AListItemMeta>
            <template #title>
              {{ item.name }}
              <ATag
                v-if="item.status"
                color="blue"
              >
                {{ $gettext('Online') }}
              </ATag>
              <ATag
                v-else
                color="error"
              >
                {{ $gettext('Offline') }}
              </ATag>
              <div class="runtime-meta">
                <template v-if="item.status">
                  <span><Icon :component="pulse" /> {{ formatDateTime(item.response_at) }}</span>
                  <span><ThunderboltOutlined />{{ item.version }}</span>
                </template>
                <span><LinkOutlined />{{ item.url }}</span>
              </div>
            </template>
            <template #avatar>
              <AAvatar :src="logo" />
            </template>
            <template #description>
              <NodeAnalyticItem :item="item" />
            </template>
          </AListItemMeta>
        </AListItem>
      </template>
    </AList>
  </ACard>
</template>

<style scoped lang="less">
.env-list-card {
  margin-top: 16px;

  .runtime-meta {
    display: inline-flex;
    @media (max-width: 700px) {
      display: block;
      margin-top: 5px;
      span {
        display: flex;
        align-items: center;
      }
    }

    span {
      font-weight: 400;
      font-size: 13px;
      margin-right: 16px;
      color: #9b9b9b;

      &.anticon {
        margin-right: 4px;
      }
    }
  }
}
</style>
