<script setup lang="ts">
import type { PluginInfo, PluginSyncPolicy } from '@/api/plugin'
import { SettingOutlined } from '@antdv-next/icons'
import { setSyncPolicy } from '@/api/plugin_sync'
import NodeSelector from '@/components/NodeSelector'
import { getErrorMessage } from '@/lib/http'

const props = defineProps<{
  plugin: PluginInfo
}>()

const emit = defineEmits<{
  updated: []
}>()

const { message } = App.useApp()

const saving = ref(false)
const popoverOpen = ref(false)
const draftNodeIds = ref<number[]>([])
const draftSyncSettings = ref(false)

const isAuto = computed(() => props.plugin.sync_policy === 'auto')
const nodeIds = computed(() => props.plugin.sync_node_ids ?? [])

// An empty node list means every child node, including the ones added later.
const targetSummary = computed(() => {
  if (!isAuto.value)
    return $gettext('Manual')
  if (nodeIds.value.length === 0)
    return $gettext('All nodes')

  return $ngettext('%{count} node', '%{count} nodes', nodeIds.value.length, {
    count: String(nodeIds.value.length),
  })
})

async function save(policy: PluginSyncPolicy, targets: number[], syncSettings: boolean) {
  saving.value = true
  try {
    await setSyncPolicy(props.plugin.id, {
      sync_policy: policy,
      sync_node_ids: targets,
      sync_settings: syncSettings,
    })
    message.success($gettext('Sync policy saved'))
    emit('updated')
  }
  catch (error) {
    message.error(getErrorMessage(error, $gettext('Failed to save the sync policy')))
  }
  finally {
    saving.value = false
  }
}

function toggle(checked: boolean) {
  save(checked ? 'auto' : 'manual', nodeIds.value, props.plugin.sync_settings)
}

function onPopoverOpenChange(open: boolean) {
  popoverOpen.value = open
  if (!open)
    return

  draftNodeIds.value = [...nodeIds.value]
  draftSyncSettings.value = props.plugin.sync_settings
}

async function saveTargets() {
  await save('auto', draftNodeIds.value, draftSyncSettings.value)
  popoverOpen.value = false
}
</script>

<template>
  <div class="flex items-center gap-2">
    <ASwitch
      size="small"
      :checked="isAuto"
      :loading="saving"
      @change="checked => toggle(Boolean(checked))"
    />

    <APopover
      :open="popoverOpen"
      trigger="click"
      placement="bottomLeft"
      :title="$gettext('Automatic installation')"
      @open-change="onPopoverOpenChange"
    >
      <template #content>
        <div class="sync-policy-popover">
          <p class="mb-2 text-gray-500">
            {{ $gettext('Keep this plugin at the same version and state on the selected child nodes. Leave every node unchecked to cover all of them, including the ones added later.') }}
          </p>

          <NodeSelector v-model:target="draftNodeIds" hidden-local />

          <ACheckbox v-model:checked="draftSyncSettings" class="mt-3">
            {{ $gettext('Also sync the plugin settings') }}
          </ACheckbox>

          <div class="mt-3 flex justify-end gap-2">
            <AButton size="small" @click="popoverOpen = false">
              {{ $gettext('Cancel') }}
            </AButton>
            <AButton
              size="small"
              type="primary"
              :loading="saving"
              @click="saveTargets"
            >
              {{ $gettext('Save') }}
            </AButton>
          </div>
        </div>
      </template>

      <AButton type="link" size="small" class="px-0">
        <template #icon>
          <SettingOutlined />
        </template>
        {{ targetSummary }}
      </AButton>
    </APopover>
  </div>
</template>

<style lang="less" scoped>
.sync-policy-popover {
  width: 360px;
  max-width: 70vw;
}
</style>
