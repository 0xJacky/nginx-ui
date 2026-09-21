<script setup lang="ts">
import InstalledList from './InstalledList.vue'
import Marketplace from './Marketplace.vue'
import PluginMatrix from './PluginMatrix.vue'

const activeKey = ref('installed')

const tabs = computed(() => [
  { key: 'installed', label: $gettext('Installed') },
  { key: 'marketplace', label: $gettext('Marketplace') },
  { key: 'nodes', label: $gettext('Nodes') },
])
</script>

<template>
  <ACard :title="$gettext('Plugins')">
    <ATabs v-model:active-key="activeKey" :items="tabs">
      <template #contentRender="{ item }">
        <InstalledList v-if="item.key === 'installed'" />
        <Marketplace v-else-if="item.key === 'marketplace'" />
        <PluginMatrix v-else-if="item.key === 'nodes'" />
      </template>
    </ATabs>
  </ACard>
</template>
