<script setup lang="ts">
import {
  CloseOutlined,
  ReloadOutlined,
  SaveOutlined,
  SettingOutlined,
} from '@ant-design/icons-vue'

interface Props {
  isConnected: boolean
  refreshing: boolean
  settingsMode: boolean
}

interface Emits {
  (e: 'refresh'): void
  (e: 'toggleSettings'): void
  (e: 'saveOrder'): void
  (e: 'cancelSettings'): void
}

defineProps<Props>()
defineEmits<Emits>()
</script>

<template>
  <div class="site-navigation-header">
    <h2 class="text-2xl font-500 text-gray-900 dark:text-gray-100 mb-4">
      {{ $gettext('Site Navigation') }}
    </h2>

    <div class="flex items-center gap-4">
      <div class="flex items-center gap-2">
        <div
          class="w-3 h-3 rounded-full"
          :class="[isConnected ? 'bg-green-500' : 'bg-red-500']"
        />
        <span class="text-sm text-gray-600 dark:text-gray-400">
          {{ isConnected ? $gettext('Connected') : $gettext('Disconnected') }}
        </span>
      </div>

      <div class="flex gap-2">
        <AButton
          v-if="settingsMode"
          type="primary"
          size="small"
          @click="$emit('saveOrder')"
        >
          <template #icon>
            <SaveOutlined />
          </template>
          {{ $gettext('Save Order') }}
        </AButton>

        <AButton
          v-if="settingsMode"
          size="small"
          @click="$emit('cancelSettings')"
        >
          <template #icon>
            <CloseOutlined />
          </template>
          {{ $gettext('Cancel') }}
        </AButton>

        <AButton
          v-if="!settingsMode"
          type="primary"
          size="small"
          :loading="refreshing"
          @click="$emit('refresh')"
        >
          <template #icon>
            <ReloadOutlined />
          </template>
          {{ $gettext('Refresh') }}
        </AButton>

        <AButton
          v-if="!settingsMode"
          size="small"
          @click="$emit('toggleSettings')"
        >
          <template #icon>
            <SettingOutlined />
          </template>
          {{ $gettext('Settings') }}
        </AButton>
      </div>
    </div>
  </div>
</template>

<style scoped>
.site-navigation-header {
  @apply flex items-center justify-between mb-6;
}

/* Responsive design */
@media (max-width: 768px) {
  .site-navigation-header {
    @apply flex-col items-start gap-4;
  }
}
</style>
