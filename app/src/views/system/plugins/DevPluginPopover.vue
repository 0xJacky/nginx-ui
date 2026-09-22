<script setup lang="ts">
import { ExperimentOutlined } from '@antdv-next/icons'
import { usePluginStore } from '@/plugin'

const { message } = useGlobalApp()
const pluginStore = usePluginStore()

const open = ref(false)
const draft = ref('')

const isActive = computed(() => Boolean(pluginStore.devPluginUrl))

function onOpenChange(value: boolean) {
  open.value = value
  if (value)
    draft.value = pluginStore.devPluginUrl
}

function apply(url: string) {
  pluginStore.devPluginUrl = url.trim()
  open.value = false
  message.success($gettext('Reload the page to apply the development plugin URL'))
}
</script>

<template>
  <APopover
    :open="open"
    trigger="click"
    placement="bottomRight"
    :title="$gettext('Development plugin URL')"
    @open-change="onOpenChange"
  >
    <template #content>
      <div class="dev-plugin-popover">
        <p class="mb-2 text-gray-500">
          {{ $gettext('Address of a plugin.json served by any static server. The plugin is loaded in addition to the installed ones.') }}
        </p>
        <AInput
          v-model:value="draft"
          placeholder="http://localhost:5173/plugin.json"
          allow-clear
          @press-enter="apply(draft)"
        />
        <div class="mt-2 flex justify-end gap-2">
          <AButton size="small" :disabled="!isActive && !draft" @click="apply('')">
            {{ $gettext('Clear') }}
          </AButton>
          <AButton size="small" type="primary" @click="apply(draft)">
            {{ $gettext('Save') }}
          </AButton>
        </div>
      </div>
    </template>

    <ATooltip :title="isActive ? pluginStore.devPluginUrl : undefined">
      <AButton :type="isActive ? 'primary' : 'default'" :ghost="isActive">
        <template #icon>
          <ExperimentOutlined />
        </template>
        {{ $gettext('Dev plugin') }}
      </AButton>
    </ATooltip>
  </APopover>
</template>

<style lang="less" scoped>
.dev-plugin-popover {
  width: 320px;
  max-width: 70vw;
}
</style>
