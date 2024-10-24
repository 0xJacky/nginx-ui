<script lang="ts" setup>
import type { Ref } from 'vue'
import VPSwitch from '@/components/VPSwitch/VPSwitch.vue'
import { useSettingsStore } from '@/pinia'
import VPIconMoon from './icons/VPIconMoon.vue'
import VPIconSun from './icons/VPIconSun.vue'

const settings = useSettingsStore()
const devicePrefersTheme = inject('devicePrefersTheme') as Ref<string>
const isDark = computed(() => settings.theme === 'dark')

const switchTitle = computed(() => {
  return isDark.value ? $gettext('Switch to light theme') : $gettext('Switch to dark theme')
})

async function toggleAppearance() {
  if (isDark.value)
    settings.set_theme('light')
  else
    settings.set_theme('dark')

  if (devicePrefersTheme.value === settings.theme)
    settings.set_preference_theme('auto')
  else
    settings.set_preference_theme(settings.theme)
}
</script>

<template>
  <VPSwitch
    :title="switchTitle"
    class="VPSwitchAppearance"
    :aria-checked="isDark"
    @click="toggleAppearance"
  >
    <VPIconSun class="sun" />
    <VPIconMoon class="moon" />
  </VPSwitch>
</template>

<style scoped>
.sun {
  opacity: 1;
}

.moon {
  opacity: 0;
}

.dark .sun {
  opacity: 0;
}

.dark .moon {
  opacity: 1;
}

.dark .VPSwitchAppearance :deep(.check) {
  /*rtl:ignore*/
  transform: translateX(18px);
}
</style>
