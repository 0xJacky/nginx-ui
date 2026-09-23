<script setup lang="ts">
import type { PluginInfo } from '@/api/plugin'
import type { PluginNodeResult } from '@/api/plugin_sync'
import { localizedPluginName } from '@/api/plugin'
import { syncPlugin } from '@/api/plugin_sync'
import NodeSelector from '@/components/NodeSelector'
import gettext from '@/gettext'
import { getErrorMessage } from '@/lib/http'
import SyncResultList from './SyncResultList.vue'

const props = defineProps<{
  plugin?: PluginInfo
}>()

const emit = defineEmits<{
  synced: []
}>()

const open = defineModel<boolean>('open', { default: false })

const { message } = useGlobalApp()

const name = computed(() => (props.plugin ? localizedPluginName(props.plugin, gettext.current) : ''))
const syncing = ref(false)
const nodeIds = ref<number[]>([])
const results = ref<PluginNodeResult[]>([])

async function run() {
  const plugin = props.plugin
  if (!plugin)
    return

  syncing.value = true
  try {
    const response = await syncPlugin(plugin.id, nodeIds.value)
    results.value = response.results
    if (response.results.every(result => result.success))
      message.success($gettext('Plugin synchronized to %{count} node(s)', { count: String(response.results.length) }))
    else
      message.warning($gettext('Some nodes could not be synchronized'))
    emit('synced')
  }
  catch (error) {
    message.error(getErrorMessage(error, $gettext('Failed to synchronize the plugin')))
  }
  finally {
    syncing.value = false
  }
}

watch(open, value => {
  if (!value)
    return

  // Start from the configured target set so a manual push matches the policy.
  nodeIds.value = [...(props.plugin?.sync_node_ids ?? [])]
  results.value = []
})
</script>

<template>
  <AModal
    v-model:open="open"
    :title="$gettext('Sync %{name} to nodes', { name })"
    :width="640"
    :ok-text="$gettext('Sync')"
    :cancel-text="$gettext('Close')"
    :confirm-loading="syncing"
    @ok="run"
  >
    <p class="mb-2 text-gray-500">
      {{ $gettext('Leave every node unchecked to sync to all child nodes.') }}
    </p>
    <NodeSelector v-model:target="nodeIds" hidden-local />

    <SyncResultList v-if="results.length > 0" :results="results" class="mt-4" />
  </AModal>
</template>
