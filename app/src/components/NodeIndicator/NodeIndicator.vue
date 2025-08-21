<script setup lang="ts">
import { CloseOutlined, DashboardOutlined, DatabaseOutlined } from '@ant-design/icons-vue'
import { storeToRefs } from 'pinia'
import { useSettingsStore } from '@/pinia'

const settingsStore = useSettingsStore()

const { node } = storeToRefs(settingsStore)
const router = useRouter()

async function clear_node() {
  await router.push('/dashboard')
  settingsStore.clear_node()
}

const isLocal = computed(() => {
  return node.value.id === 0
})

const nodeId = computed(() => node.value.id)

watch(nodeId, async () => {
  await router.push('/dashboard')
  location.reload()
})

const { server_name } = storeToRefs(useSettingsStore())
</script>

<template>
  <div class="indicator">
    <div class="container">
      <DatabaseOutlined />
      <span
        v-if="isLocal"
        class="node-name"
      >
        {{ server_name || $gettext('Local') }}
      </span>
      <span
        v-else
        class="node-name"
      >
        {{ node.name }}
      </span>
      <ATag @click="clear_node">
        <DashboardOutlined v-if="isLocal" />
        <CloseOutlined v-else />
      </ATag>
    </div>
  </div>
</template>

<style scoped lang="less">
.ant-layout-sider-collapsed {
  .ant-tag, .node-name {
    display: none;
  }

  .indicator {
    .container {
      justify-content: center;
    }
  }
}

.indicator {
  padding: 20px 20px 16px 20px;

  .container {
    border-radius: 16px;
    border: 1px solid #91d5ff;
    background: #e6f7ff;
    padding: 5px 15px;
    color: #096dd9;

    display: flex;
    align-items: center;
    justify-content: space-between;

    .node-name {
      max-width: 85px;
      text-overflow: ellipsis;
      white-space: nowrap;
      line-height: 1em;
      overflow: hidden;
    }

    .ant-tag {
      cursor: pointer;
      margin-right: 0;
      padding: 0 5px;
    }
  }
}

.dark {
  .container {
    border: 1px solid #545454;
    background: transparent;
    color: #bebebe;
  }
}
</style>
