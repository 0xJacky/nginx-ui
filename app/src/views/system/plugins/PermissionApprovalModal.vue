<script setup lang="ts">
import type { PluginInfo } from '@/api/plugin'
import { localizedPluginName } from '@/api/plugin'
import gettext from '@/gettext'
import PermissionList from './PermissionList.vue'
import { permissionReasons } from './permissions'

const props = defineProps<{
  plugin?: PluginInfo
  confirmLoading?: boolean
}>()

const emit = defineEmits<{
  approve: []
}>()

const open = defineModel<boolean>('open', { default: false })

const permissions = computed(() => props.plugin?.permissions ?? [])
const reasons = computed(() => permissionReasons(props.plugin, gettext.current))
const name = computed(() => (props.plugin ? localizedPluginName(props.plugin, gettext.current) : ''))
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
      :title="$gettext('%{name} needs your approval before it can run.', { name })"
      :description="$gettext('Approve only if you trust the author. The approval is recorded and asked again whenever the requested permissions change.')"
    />

    <PermissionList :permissions="permissions" :reasons="reasons" />
  </AModal>
</template>
