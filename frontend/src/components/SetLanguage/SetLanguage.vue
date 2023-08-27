<script setup lang="ts">
import gettext from '@/gettext'

import {ref, watch} from 'vue'

import {useSettingsStore} from '@/pinia'
import {useRoute} from 'vue-router'
import http from '@/lib/http'

const settings = useSettingsStore()

const route = useRoute()

const current = ref(gettext.current)

const languageAvailable = gettext.available

function init() {
    if (current.value !== 'en') {
        http.get('/translation/' + current.value).then(r => {
            gettext.translations[current.value] = r
        })
    }
}

init()

watch(current, (v) => {
    init()
    settings.set_language(v)
    gettext.current = v
    // @ts-ignored
    document.title = route.name() + ' | Nginx UI'
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
