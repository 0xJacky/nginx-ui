<script setup lang="ts">
const props = withDefaults(defineProps<{
  src?: string
  /** Display name, its first letter stands in when there is no image. */
  name?: string
  size?: number
  /** Drains the colour, for a plugin that is switched off. */
  muted?: boolean
}>(), {
  size: 36,
})

const failed = ref(false)
const loaded = ref(false)

// A new source gets a fresh chance to load.
watch(() => props.src, () => {
  failed.value = false
  loaded.value = false
})

const tryImage = computed(() => Boolean(props.src) && !failed.value)
// The letter stays until the image has actually loaded, so a slow or
// unreachable icon never leaves an empty square.
const showImage = computed(() => tryImage.value && loaded.value)

const initial = computed(() => {
  const letter = Array.from(props.name?.trim() ?? '')[0] ?? '?'
  return letter.toLocaleUpperCase()
})

const style = computed(() => ({
  width: `${props.size}px`,
  height: `${props.size}px`,
  fontSize: `${Math.round(props.size * 0.45)}px`,
  borderRadius: `${Math.round(props.size * 0.28)}px`,
}))
</script>

<template>
  <span
    class="plugin-icon"
    :class="{ 'is-letter': !showImage, 'is-muted': props.muted }"
    :style="style"
    aria-hidden="true"
  >
    <template v-if="!showImage">{{ initial }}</template>
    <img
      v-if="tryImage"
      :class="{ 'is-pending': !showImage }"
      :src="props.src"
      alt=""
      loading="lazy"
      @load="loaded = true"
      @error="failed = true"
    >
  </span>
</template>

<style lang="less" scoped>
.plugin-icon {
  flex: none;
  display: inline-flex;
  align-items: center;
  justify-content: center;
  overflow: hidden;
  background: var(--ant-color-fill-tertiary);
  transition: filter 0.2s ease;

  position: relative;

  img {
    width: 100%;
    height: 100%;
    object-fit: contain;

    &.is-pending {
      position: absolute;
      inset: 0;
      opacity: 0;
    }
  }

  &.is-letter {
    font-weight: 600;
    line-height: 1;
    color: var(--ant-color-primary);
    background: var(--ant-color-primary-bg);
    user-select: none;
  }

  &.is-muted {
    filter: grayscale(1);
    opacity: 0.7;
  }
}

@media (prefers-reduced-motion: reduce) {
  .plugin-icon {
    transition: none;
  }
}
</style>
