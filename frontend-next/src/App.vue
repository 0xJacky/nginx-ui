<script setup lang="ts">
// This starter template is using Vue 3 <script setup> SFCs
// Check out https://vuejs.org/api/sfc-script-setup.html#script-setup
//@ts-ignore
import {useSettingsStore} from "@/pinia/settings"
import {dark_mode} from "@/lib/theme"

let media = window.matchMedia('(prefers-color-scheme: dark)')
const callback = (media: { matches: any; }) => {
    const settings = useSettingsStore()
    if (media.matches) {
        dark_mode(true)
        settings.set_theme('dark')
    } else {
        dark_mode(false)
        settings.set_theme('default')
    }
}
callback(media)
if (typeof media.addEventListener === 'function') {
    media.addEventListener('change', callback)
} else if (typeof media.addListener === 'function') {
    media.addListener(callback)
}

</script>

<template>
    <router-view/>
</template>

<style lang="less" scoped>
#app {
    height: 100%;
}
</style>
