<script setup lang="ts">
import type { Node } from '@/api/environment'
import type ReconnectingWebSocket from 'reconnecting-websocket'
import type { Ref } from 'vue'
import analytic from '@/api/analytic'
import environment from '@/api/environment'
import logo from '@/assets/img/logo.png'
import pulse from '@/assets/svg/pulse.svg?component'
import { formatDateTime } from '@/lib/helper'
import { useSettingsStore } from '@/pinia'
import { version } from '@/version.json'
import NodeAnalyticItem from '@/views/dashboard/components/NodeAnalyticItem.vue'
import Icon, { LinkOutlined, SendOutlined, ThunderboltOutlined } from '@ant-design/icons-vue'

const data = ref([]) as Ref<Node[]>

const nodeMap = computed(() => {
  const o = {} as Record<number, Node>

  data.value.forEach(v => {
    o[v.id] = v
  })

  return o
})

let websocket: ReconnectingWebSocket | WebSocket

onMounted(async () => {
  let hasMore = true
  let page = 1
  while (hasMore) {
    await environment.get_list({ page, enabled: true }).then(r => {
      data.value.push(...r.data)
      hasMore = r.data.length === r.pagination.per_page
      page++
    }).catch(() => {
      hasMore = false
    })
  }
})

onMounted(() => {
  websocket = analytic.nodes()
  websocket.onmessage = async m => {
    const nodes = JSON.parse(m.data)

    Object.keys(nodes).forEach((v: string) => {
      const key = Number.parseInt(v)

      // update node online status
      if (nodeMap.value[key]) {
        Object.assign(nodeMap.value[key], nodes[key])
        nodeMap.value[key].response_at = new Date()
      }
    })
  }
})

onUnmounted(() => {
  websocket.close()
})

const { environment: env } = useSettingsStore()

function linkStart(node: Node) {
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
          <AListItemMeta>
            <template #title>
              <div class="mb-1">
                {{ item.name }}
                <ATag
                  v-if="item.status"
                  color="blue"
                  class="ml-2"
                >
                  {{ $gettext('Online') }}
                </ATag>
                <ATag
                  v-else
                  color="error"
                  class="ml-2"
                >
                  {{ $gettext('Offline') }}
                </ATag>
              </div>

              <template v-if="item.status">
                <div class="runtime-meta mr-2 mb-1">
                  <Icon :component="pulse" /> {{ formatDateTime(item.response_at) }}
                </div>
                <div class="runtime-meta mr-2 mb-1">
                  <ThunderboltOutlined />{{ item.version }}
                </div>
              </template>
              <div class="runtime-meta">
                <LinkOutlined />{{ item.url }}
              </div>
            </template>
            <template #avatar>
              <AAvatar :src="logo" />
            </template>
            <template #description>
              <div class="md:flex lg:flex justify-between md:items-center">
                <NodeAnalyticItem
                  :item="item"
                  class="mt-1 mb-1"
                />

                <AButton
                  v-if="item.version === version"
                  type="primary"
                  :disabled="!item.status || env.id === item.id"
                  ghost
                  @click="linkStart(item)"
                >
                  <SendOutlined />
                  {{ env.id !== item.id ? $gettext('Link Start') : $gettext('Connected') }}
                </AButton>
                <ATooltip
                  v-else
                  placement="topLeft"
                >
                  <template #title>
                    {{ $gettext('The remote Nginx UI version is not compatible with the local Nginx UI version. '
                      + 'To avoid potential errors, please upgrade the remote Nginx UI to match the local version.') }}
                  </template>
                  <AButton
                    ghost
                    disabled
                  >
                    <SendOutlined />
                    {{ $gettext('Link Start') }}
                  </AButton>
                </ATooltip>
              </div>
            </template>
          </AListItemMeta>
        </AListItem>
      </template>
    </AList>
  </ACard>
</template>

<style scoped lang="less">
:deep(.ant-list-item-meta-title) {
  display: flex;
  align-items: center;
  @media (max-width: 700px) {
    display: block;
  }
}

.env-list-card {
  margin-top: 16px;

  .runtime-meta {
    display: inline-flex;
    @media (max-width: 700px) {
      align-items: center;
    }
    font-weight: 400;
    color: #9b9b9b;

    .anticon {
      margin-right: 4px;
    }
  }
}

:deep(.ant-list-item-action) {
  @media(max-width: 500px) {
    display: none;
  }
}
</style>
