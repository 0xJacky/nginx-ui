<script setup lang="ts">
import { CopyOutlined } from '@antdv-next/icons'
import { useClipboard } from '@vueuse/core'

// The include line of a file nginx-ui generates, such as a blocklist or a
// discovered upstream, with a button that copies it. nginx resolves the
// relative path against the directory of nginx.conf.
const props = defineProps<{
  include?: string
  path?: string
}>()

const { message } = useGlobalApp()
const { copy } = useClipboard()

async function handleCopy() {
  if (!props.include)
    return
  try {
    await copy(props.include)
    message.success($gettext('Include line copied to clipboard'))
  }
  catch (error) {
    console.error(error)
    message.error($gettext('Failed to copy to clipboard'))
  }
}
</script>

<template>
  <div
    v-if="props.include"
    class="flex items-center gap-1 min-w-0"
  >
    <ATooltip :title="props.path">
      <code class="generated-include truncate">{{ props.include }}</code>
    </ATooltip>
    <ATooltip :title="$gettext('Copy')">
      <AButton
        type="text"
        size="small"
        :aria-label="$gettext('Copy')"
        @click="handleCopy"
      >
        <template #icon>
          <CopyOutlined />
        </template>
      </AButton>
    </ATooltip>
  </div>
</template>

<style scoped>
.generated-include {
  font-size: 12px;
  padding: 1px 6px;
  border-radius: 4px;
  background: rgba(0, 0, 0, 0.04);
}

.dark .generated-include {
  background: rgba(255, 255, 255, 0.08);
}
</style>
