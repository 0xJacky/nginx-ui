<script setup lang="ts">
// This starter template is using Vue 3 <script setup> SFCs
// Check out https://vuejs.org/api/sfc-script-setup.html#script-setup
import {toggleTheme} from '@zougt/vite-plugin-theme-preprocessor/dist/browser-utils.js'

let media = window.matchMedia('(prefers-color-scheme: dark)')
const callback = (media: { matches: any; }) => {
    if (media.matches) {
        toggleTheme({
            scopeName: 'theme-dark'
        })
    } else {
        toggleTheme({
            scopeName: 'theme-default'
        })
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
