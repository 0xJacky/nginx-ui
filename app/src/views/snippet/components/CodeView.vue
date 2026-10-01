<script setup lang="ts">
import { colorOf, segments } from '../template'

const props = defineProps<{
  content: string
  // Color slot of each declared variable; without it actions are not colored.
  slots?: Map<string, number>
  // Line numbers, from 1, to mark as changed.
  changed?: Set<number>
}>()

const lines = computed(() => {
  const result: { text: string, key?: string, isAction: boolean }[][] = [[]]
  for (const part of props.slots ? segments(props.content) : [{ text: props.content, isAction: false }]) {
    part.text.split('\n').forEach((text, i) => {
      if (i > 0)
        result.push([])
      if (text)
        result.at(-1)!.push({ ...part, text })
    })
  }
  if (result.length > 1 && result.at(-1)!.length === 0)
    result.pop()
  return result
})
</script>

<template>
  <div class="code-view">
    <div
      v-for="(line, index) in lines"
      :key="index"
      class="line"
      :class="{ changed: changed?.has(index + 1) }"
    >
      <span class="number">{{ index + 1 }}</span>
      <span class="text"><template
        v-for="(part, i) in line"
        :key="i"
      ><span
        v-if="part.isAction && part.key && slots"
        class="action"
        :style="{ '--c': colorOf(slots, part.key) }"
      >{{ part.text }}</span><template v-else>{{ part.text }}</template></template></span>
    </div>
  </div>
</template>

<style scoped lang="less">
.code-view {
  background: #272822;
  color: #f8f8f2;
  border-radius: 6px;
  padding: 8px 0;
  font: 13px/21px ui-monospace, SFMono-Regular, Menlo, Consolas, monospace;
  overflow-x: auto;
}

.line {
  display: flex;
  min-width: max-content;
  padding-right: 12px;

  &.changed {
    background: rgba(166, 226, 46, 0.16);
  }
}

.number {
  flex: none;
  width: 40px;
  padding-right: 10px;
  text-align: right;
  color: #8f908a;
  user-select: none;
}

.text {
  white-space: pre;
}

.action {
  border-radius: 3px;
  background: color-mix(in srgb, var(--c) 30%, transparent);
  box-shadow: inset 0 -2px 0 var(--c);
}
</style>
