<script setup lang="ts">
import {VAceEditor} from 'vue3-ace-editor'
import 'ace-builds/src-noconflict/mode-nginx'
import 'ace-builds/src-noconflict/theme-monokai'

const {content} = defineProps(['content'])
</script>

<template>
    <v-ace-editor
        v-model:value="content"
        lang="nginx"
        theme="monokai"
        style="height: 300px"/>
</template>

<style scoped>

</style>
