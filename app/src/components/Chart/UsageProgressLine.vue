<script setup lang="ts">
import { computed } from 'vue'

const props = withDefaults(defineProps<{
  percent?: number
}>(), {
  percent: 0,
})

const color = computed(() => {
  if (props.percent < 80)
    return '#1890ff'
  else if (props.percent >= 80 && props.percent < 90)
    return '#faad14'
  else
    return '#ff6385'
})

const fixed_percent = computed(() => {
  return Number.parseFloat(props.percent.toFixed(2))
})
</script>

<template>
  <div>
    <div class="flex items-center">
      <span class="slot-icon"><slot name="icon" /></span>
      <span class="slot">
        <slot />
      </span>
      <span class="dot mx-2">·</span>{{ `${fixed_percent}%` }}
    </div>
    <AProgress
      :percent="fixed_percent"
      :stroke-color="color"
      :show-info="false"
    />
  </div>
</template>

<style scoped lang="less">
.slot-icon {
  margin-right: 5px;
  display: flex;
  align-items: center;
}
</style>
