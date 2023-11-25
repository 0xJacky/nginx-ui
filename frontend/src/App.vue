<script setup lang="ts">
// This starter template is using Vue 3 <script setup> SFCs
// Check out https://vuejs.org/api/sfc-script-setup.html#script-setup
import {useSettingsStore} from '@/pinia'
import {computed, provide} from 'vue'

const media = window.matchMedia('(prefers-color-scheme: dark)')

const callback = (media: { matches: any; }) => {
  const settings = useSettingsStore()
  if (settings.preference_theme === 'auto') {
    if (media.matches) {
      settings.set_theme('dark')
    } else {
      settings.set_theme('light')
    }
  } else {
    settings.set_theme(settings.preference_theme)
  }
}

callback(media)

const devicePrefersTheme = computed(() => {
  return media.matches ? 'dark' : 'light'
})

provide('devicePrefersTheme', devicePrefersTheme)

media.addEventListener('change', callback)
</script>

<template>
  <router-view/>
</template>

<style lang="less">
@import "ant-design-vue/dist/reset.css";
</style>

<style lang="less" scoped>

</style>
