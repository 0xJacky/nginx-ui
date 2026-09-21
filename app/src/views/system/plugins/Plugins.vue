<script setup lang="ts">
import InstalledList from './InstalledList.vue'
import MarketplacePlaceholder from './MarketplacePlaceholder.vue'

const activeKey = ref('installed')

const tabs = computed(() => [
  { key: 'installed', label: $gettext('Installed') },
  { key: 'marketplace', label: $gettext('Marketplace') },
])
</script>

<template>
  <ACard :title="$gettext('Plugins')">
    <ATabs v-model:active-key="activeKey" :items="tabs">
      <template #contentRender="{ item }">
        <InstalledList v-if="item.key === 'installed'" />
        <MarketplacePlaceholder v-else-if="item.key === 'marketplace'" />
      </template>
    </ATabs>
  </ACard>
</template>
