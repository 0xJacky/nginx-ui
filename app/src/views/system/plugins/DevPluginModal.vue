<script setup lang="ts">
import { isLoopbackUrl, usePluginStore } from '@/plugin'

const open = defineModel<boolean>('open', { default: false })

const { message } = useGlobalApp()
const pluginStore = usePluginStore()

const draft = ref('')

const isActive = computed(() => Boolean(pluginStore.devPluginUrl))

watch(open, value => {
  if (value)
    draft.value = pluginStore.devPluginUrl
})

function apply(url: string) {
  const value = url.trim()

  // The loader injects this script on every visit, so only this computer may serve it.
  if (value && !isLoopbackUrl(value)) {
    message.error($gettext('Only localhost addresses are accepted'))
    return
  }

  pluginStore.devPluginUrl = value
  open.value = false
  message.success($gettext('Reload the page to apply the development plugin URL'))
}
</script>

<template>
  <AModal
    v-model:open="open"
    :title="$gettext('Development plugin URL')"
    :footer="null"
    :width="480"
    destroy-on-hidden
  >
    <p class="mb-3 text-gray-500">
      {{ $gettext('Address of a plugin.json served by a static server on this computer. Only localhost addresses are accepted. The plugin is loaded in addition to the installed ones.') }}
    </p>
    <AInput
      v-model:value="draft"
      placeholder="http://localhost:5173/plugin.json"
      allow-clear
      @press-enter="apply(draft)"
    />
    <div class="mt-4 flex justify-end gap-2">
      <AButton :disabled="!isActive && !draft" @click="apply('')">
        {{ $gettext('Clear') }}
      </AButton>
      <AButton type="primary" @click="apply(draft)">
        {{ $gettext('Save') }}
      </AButton>
    </div>
  </AModal>
</template>
