<script setup lang="ts">
import { AppstoreOutlined } from '@antdv-next/icons'

const props = withDefaults(defineProps<{
  src?: string
  size?: number
}>(), {
  size: 36,
})

const style = computed(() => ({
  width: `${props.size}px`,
  height: `${props.size}px`,
  fontSize: `${Math.round(props.size * 0.5)}px`,
  borderRadius: `${Math.round(props.size * 0.28)}px`,
}))
</script>

<template>
  <span class="plugin-icon" :style="style" aria-hidden="true">
    <img
      v-if="props.src"
      :src="props.src"
      alt=""
      loading="lazy"
    >
    <AppstoreOutlined v-else />
  </span>
</template>

<style lang="less" scoped>
.plugin-icon {
  flex: none;
  display: inline-flex;
  align-items: center;
  justify-content: center;
  overflow: hidden;
  color: var(--ant-color-text-tertiary);
  background: var(--ant-color-fill-tertiary);

  img {
    width: 100%;
    height: 100%;
    object-fit: contain;
  }
}
</style>
