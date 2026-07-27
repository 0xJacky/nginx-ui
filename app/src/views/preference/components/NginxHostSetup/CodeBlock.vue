<script setup lang="ts">
import { CheckOutlined, CopyOutlined } from '@ant-design/icons-vue'
import { useClipboard } from '@vueuse/core'
import { computed } from 'vue'

interface Props {
  code: string
  language?: string
  title?: string
  /** Rendered above the code, outside the copyable area, so it stays translatable. */
  description?: string
  /** Shown as a numbered badge when the snippet is one of an ordered set. */
  order?: number
}

const props = withDefaults(defineProps<Props>(), {
  language: 'shell',
  title: '',
  description: '',
  order: 0,
})

const { copy, copied, isSupported } = useClipboard({ legacy: true })
const { message } = useGlobalApp()
const trimmed = computed(() => props.code.trimEnd())
const heading = computed(() => props.title || props.language)

async function copyCode() {
  if (!isSupported.value) {
    message.error($gettext('Clipboard access is unavailable in this browser context'))
    return
  }
  await copy(trimmed.value)
}
</script>

<template>
  <ACard size="small" class="code-block" :body-style="{ padding: 0 }">
    <template #title>
      <ASpace :size="6" wrap>
        <ATag v-if="order" color="processing" :bordered="false">
          {{ order }}
        </ATag>
        <span class="break-all text-xs font-medium">{{ heading }}</span>
      </ASpace>
    </template>
    <template #extra>
      <AButton size="small" :type="copied ? 'primary' : 'default'" @click="copyCode">
        <CheckOutlined v-if="copied" />
        <CopyOutlined v-else />
        {{ copied ? $gettext('Copied') : $gettext('Copy') }}
      </AButton>
    </template>
    <ATypographyText v-if="description" type="secondary" class="code-block__description">
      {{ description }}
    </ATypographyText>
    <pre class="code-block__body"><code>{{ trimmed }}</code></pre>
  </ACard>
</template>

<style lang="less" scoped>
.code-block__description {
  display: block;
  padding: 10px 12px 0;
  font-size: 12px;
  line-height: 1.5;
}

.code-block__body {
  margin: 0;
  padding: 12px;
  overflow-x: auto;
  font-size: 13px;
  line-height: 1.5;
}

// Narrow screens cannot scroll a long snippet comfortably, so wrap instead.
@media (max-width: 600px) {
  .code-block__body {
    padding: 8px;
    overflow-x: visible;
    font-size: 12px;
    white-space: pre-wrap;
    word-break: break-all;
  }
}
</style>
