<script setup lang="ts">
import type ReconnectingWebSocket from 'reconnecting-websocket'
import type { Ref } from 'vue'
import type { Node } from '@/api/environment'
import Icon, { LinkOutlined, ThunderboltOutlined } from '@ant-design/icons-vue'
import analytic from '@/api/analytic'
import environment from '@/api/environment'
import logo from '@/assets/img/logo.png'
import pulse from '@/assets/svg/pulse.svg?component'
import { formatDateTime } from '@/lib/helper'
import { useSettingsStore } from '@/pinia'
import { version } from '@/version.json'
import NodeAnalyticItem from './components/NodeAnalyticItem.vue'

const data = ref([]) as Ref<Node[]>

const nodeMap = computed(() => {
  const o = {} as Record<number, Node>

  data.value.forEach(v => {
    o[v.id] = v
  })

  return o
})

let websocket: ReconnectingWebSocket | WebSocket

onMounted(() => {
  environment.getList({ enabled: true }).then(r => {
    data.value.push(...r.data)
  })
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
    class="env-list-card w-full max-w-none"
    :title="$gettext('Environments')"
    :bordered="false"
  >
    <AList
      item-layout="horizontal"
      :data-source="data"
      class="env-list"
    >
      <template #renderItem="{ item }">
        <AListItem class="env-list-item">
          <AListItemMeta>
            <template #title>
              <div class="env-title-wrapper">
                <div class="env-tags">
                  <ATag
                    v-if="item.status"
                    color="blue"
                    :bordered="false"
                  >
                    {{ $gettext('Online') }}
                  </ATag>
                  <ATag
                    v-else
                    color="error"
                    :bordered="false"
                  >
                    {{ $gettext('Offline') }}
                  </ATag>
                </div>
                <span class="env-name">{{ item.name }}</span>
              </div>

              <div class="env-meta-wrapper">
                <template v-if="item.status">
                  <div class="runtime-meta">
                    <Icon :component="pulse" />
                    <span>{{ formatDateTime(item.response_at) }}</span>
                  </div>
                  <div class="runtime-meta">
                    <ThunderboltOutlined />
                    <span>{{ item.version }}</span>
                  </div>
                </template>
                <div class="runtime-meta">
                  <LinkOutlined />
                  <span class="truncate">{{ item.url }}</span>
                </div>
              </div>
            </template>
            <template #avatar>
              <AAvatar :src="logo" class="flex-shrink-0" />
            </template>
            <template #description>
              <div class="env-description">
                <NodeAnalyticItem
                  :item="item"
                  :current-env-id="env.id"
                  :local-version="version"
                  :on-link-start="linkStart"
                  class="node-analytic"
                />
              </div>
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

  // Ensure card doesn't overflow on small screens
  @media (max-width: 768px) {
    margin-left: -8px;
    margin-right: -8px;
    border-radius: 0;
  }
}

.env-list {
  // Responsive handling for list container
  .env-list-item {
    padding: 16px 0;

    @media (max-width: 576px) {
      padding: 12px 0;
    }
  }
}

// Title area styles
.env-title-wrapper {
  display: flex;
  align-items: center;
  gap: 4px;
  flex-wrap: wrap;
  margin-right: 8px;

  .env-name {
    font-weight: 500;
    line-height: 1.4;
  }
}

// Metadata area styles
.env-meta-wrapper {
  display: flex;
  flex-wrap: wrap;
  gap: 12px;

  @media (max-width: 768px) {
    margin-top: 8px;
    margin-bottom: 8px;
  }

  @media (max-width: 576px) {
    gap: 8px;
  }
}

.runtime-meta {
  display: inline-flex;
  align-items: center;
  gap: 4px;
  font-weight: 400;
  color: #9b9b9b;
  font-size: 14px;
  line-height: 1.4;
  max-width: 100%;

  .anticon {
    flex-shrink: 0;
  }

  span {
    min-width: 0; // Allow text truncation
  }

  @media (max-width: 576px) {
    font-size: 12px;
  }
}

// Description area styles
.env-description {
  margin-top: 8px;

  .node-analytic {
    width: 100%;
  }
}

// Global dark mode class adaptation
.dark {
  .ant-list-item-meta-avatar .ant-avatar {
    border: 1px solid #303030 !important;
  }
}

// Deep selector optimizations
:deep(.ant-list-item-meta) {
  width: 100%;
  display: flex;

  .ant-list-item-meta-content {
    width: 100%;
    min-width: 0; // Allow content to shrink
  }

  .ant-list-item-meta-title {
    margin-bottom: 0;
    display: flex;
    align-items: center;

    @media (max-width: 768px) {
      display: block;
    }
  }

  .ant-list-item-meta-avatar {
    display: flex;
    align-items: center;
    align-self: center; // Vertically center relative to the entire meta container

    .ant-avatar {
      border: 1px solid #f0f0f0;
      border-radius: 8px; // Square with rounded corners
      padding: 2px; // Add padding
      transition: border-color 0.2s ease;
    }
    @media (max-width: 768px) {
      display: none;
    }
  }
}

// Responsive breakpoint optimizations
@media (max-width: 1200px) {
  .env-meta-wrapper {
    gap: 10px;
  }
}

@media (max-width: 992px) {
  .env-title-wrapper .env-name {
    font-size: 15px;
  }

  .runtime-meta {
    font-size: 13px;
  }
}

@media (max-width: 480px) {
  .env-title-wrapper {
    margin-bottom: 6px;
  }

  .env-meta-wrapper {
    gap: 6px;
  }

  .runtime-meta {
    font-size: 11px;
  }
}
</style>
