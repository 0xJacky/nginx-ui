<script setup lang="ts">
import {VAceEditor} from 'vue3-ace-editor'
import 'ace-builds/src-noconflict/mode-nginx'
import 'ace-builds/src-noconflict/theme-monokai'

const {content, defaultHeight} = defineProps<{
    content?: string
    defaultHeight?: string
}>()
</script>

<template>
    <v-ace-editor
        v-model:value="content"
        lang="nginx"
        theme="monokai"
        :style="{
            minHeight: defaultHeight || '100vh'
        }"/>
</template>

<style scoped>

</style>
