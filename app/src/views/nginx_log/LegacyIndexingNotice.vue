<script setup lang="ts">
import nginxLog from '@/api/nginx_log'
import pluginApi from '@/api/plugin'
import { logAnalyticsInstallRoute, needsLogAnalyticsPlugin } from './legacyIndexing'

const router = useRouter()
const visible = ref(false)

onMounted(async () => {
  try {
    const status = await nginxLog.getLegacyIndexingStatus()
    if (!status.enabled)
      return

    let installedIds: string[] | null = null
    try {
      installedIds = (await pluginApi.getList()).map(item => item.id)
    }
    catch {
      installedIds = null
    }
    visible.value = needsLogAnalyticsPlugin(status.enabled, installedIds)
  }
  catch {
    visible.value = false
  }
})
</script>

<template>
  <AAlert
    v-if="visible"
    class="mb-4"
    type="info"
    show-icon
    :title="$gettext('Advanced Indexing is now provided by the Log Analytics plugin. Install the plugin to continue using it. Existing indexes are kept.')"
  >
    <template #action>
      <AButton size="small" type="primary" @click="router.push(logAnalyticsInstallRoute())">
        {{ $gettext('Go to install') }}
      </AButton>
    </template>
  </AAlert>
</template>
