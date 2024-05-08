<script setup lang="ts">
import { CloseOutlined, DashboardOutlined, DatabaseOutlined } from '@ant-design/icons-vue'
import { storeToRefs } from 'pinia'
import { useRouter } from 'vue-router'
import { computed, watch } from 'vue'
import { useSettingsStore } from '@/pinia'

const settingsStore = useSettingsStore()

const { environment } = storeToRefs(settingsStore)
const router = useRouter()

async function clear_env() {
  await router.push('/dashboard')
  settingsStore.clear_environment()
}

const is_local = computed(() => {
  return environment.value.id === 0
})

const node_id = computed(() => environment.value.id)

watch(node_id, async () => {
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
        v-if="is_local"
        class="env-name"
      >
        {{ server_name || $gettext('Local') }}
      </span>
      <span
        v-else
        class="env-name"
      >
        {{ environment.name }}
      </span>
      <ATag @click="clear_env">
        <DashboardOutlined v-if="is_local" />
        <CloseOutlined v-else />
      </ATag>
    </div>
  </div>
</template>

<style scoped lang="less">
.ant-layout-sider-collapsed {
  .ant-tag, .env-name {
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

    .env-name {
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
