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
          @click="$emit('saveOrder')"
        >
          <template #icon>
            <SaveOutlined />
          </template>
          {{ $gettext('Save Order') }}
        </AButton>

        <AButton
          v-if="settingsMode"
          @click="$emit('cancelSettings')"
        >
          <template #icon>
            <CloseOutlined />
          </template>
        </AButton>

        <AButton
          v-if="!settingsMode"
          type="primary"
          :loading="refreshing"
          @click="$emit('refresh')"
        >
          <template #icon>
            <ReloadOutlined />
          </template>
        </AButton>

        <AButton
          v-if="!settingsMode"
          @click="$emit('toggleSettings')"
        >
          <template #icon>
            <SettingOutlined />
          </template>
        </AButton>
      </div>
    </div>
  </div>
</template>

<style scoped>
.site-navigation-header {
  @apply flex items-center justify-end;
}

/* Responsive design */
@media (max-width: 768px) {
  .site-navigation-header {
    @apply flex-col items-start gap-4;
  }
}
</style>
