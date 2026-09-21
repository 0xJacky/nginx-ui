<script setup lang="ts">
import type { PluginInfo } from '@/api/plugin'
import PermissionList from './PermissionList.vue'

const props = defineProps<{
  plugin?: PluginInfo
  confirmLoading?: boolean
}>()

const emit = defineEmits<{
  approve: []
}>()

const open = defineModel<boolean>('open', { default: false })

const permissions = computed(() => props.plugin?.permissions ?? [])
</script>

<template>
  <AModal
    v-model:open="open"
    :title="$gettext('Approve permissions')"
    :ok-text="$gettext('Approve and enable')"
    :cancel-text="$gettext('Cancel')"
    :confirm-loading="props.confirmLoading"
    :width="560"
    @ok="emit('approve')"
  >
    <AAlert
      type="warning"
      show-icon
      class="mb-4"
      :title="$gettext('%{name} needs your approval before it can run.', { name: props.plugin?.name ?? '' })"
      :description="$gettext('Approve only if you trust the author. The approval is recorded and asked again whenever the requested permissions change.')"
    />

    <PermissionList :permissions="permissions" />
  </AModal>
</template>
