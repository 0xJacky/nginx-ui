<script setup lang="ts">
import { BulbOutlined, DownOutlined, LoadingOutlined, RightOutlined } from '@antdv-next/icons'

const props = defineProps<{
  reasoning: string
  // The model is still thinking: reasoning streams in, no answer yet
  thinking: boolean
}>()

// Open while the model thinks, folded once the answer starts, unless the
// user opened or closed it by hand.
const userExpanded = ref<boolean>()
const isExpanded = computed(() => userExpanded.value ?? props.thinking)

const bodyRef = useTemplateRef<HTMLElement>('body')

// Follow the newest reasoning while it streams in
watch(() => props.reasoning, async () => {
  if (!props.thinking || !isExpanded.value)
    return
  await nextTick()
  bodyRef.value?.scrollTo({ top: bodyRef.value.scrollHeight })
})

function toggle() {
  userExpanded.value = !isExpanded.value
}
</script>

<template>
  <div class="mb-2">
    <button
      type="button"
      class="reasoning-toggle"
      :aria-expanded="isExpanded"
      @click="toggle"
    >
      <LoadingOutlined v-if="thinking" />
      <BulbOutlined v-else />
      <span>{{ thinking ? $gettext('Thinking...') : $gettext('Thought process') }}</span>
      <DownOutlined v-if="isExpanded" class="text-[10px]" />
      <RightOutlined v-else class="text-[10px]" />
    </button>
    <div
      v-show="isExpanded"
      ref="body"
      class="reasoning-body"
    >
      {{ reasoning }}
    </div>
  </div>
</template>

<style lang="less" scoped>
.reasoning-toggle {
  display: inline-flex;
  align-items: center;
  gap: 6px;
  padding: 0;
  border: 0;
  background: none;
  font: inherit;
  font-size: 12px;
  // Inherit the panel's text color: the terminal assistant is dark in both
  // themes, so theme tokens would put dark text on a dark background
  color: inherit;
  opacity: .55;
  cursor: pointer;
  transition: opacity .3s;

  &:hover {
    opacity: .8;
  }

  &:focus-visible {
    outline: 2px solid var(--ant-color-primary);
    outline-offset: 2px;
    border-radius: 4px;
  }
}

.reasoning-body {
  margin-top: 6px;
  padding: 2px 0 2px 10px;
  border-left: 2px solid color-mix(in srgb, currentColor 20%, transparent);
  max-height: 240px;
  overflow-y: auto;
  white-space: pre-wrap;
  word-break: break-word;
  font-size: 12px;
  line-height: 1.6;
  color: color-mix(in srgb, currentColor 70%, transparent);
}
</style>
