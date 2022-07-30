<script setup lang="ts">
import gettext from "@/gettext"


import {ref, watch, nextTick} from "vue"

import {useSettingsStore} from "@/pinia/settings"
const settings = useSettingsStore()


const current = ref(gettext.current)

const languageAvailable = gettext.available
watch(current, (v) => {
    settings.set_language(v)
    gettext.current = v
    // nextTick(() => {
    //     location.reload()
    // })
})

</script>

<template>
    <div>
        <a-select v-model:value="current" size="small" style="width: 60px">
            <a-select-option v-for="(language, key) in languageAvailable" :value="key" :key="key">
                {{ language }}
            </a-select-option>
        </a-select>
    </div>
</template>

<style lang="less" scoped>

</style>
