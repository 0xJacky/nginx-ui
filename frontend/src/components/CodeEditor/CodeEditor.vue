<script setup lang="ts">
import {VAceEditor} from 'vue3-ace-editor'
import 'ace-builds/src-noconflict/mode-nginx'
import 'ace-builds/src-noconflict/theme-monokai'
import {computed} from 'vue'

const props = defineProps(['content', 'defaultHeight'])

const emit = defineEmits(['update:content'])

const value = computed({
    get() {
        return props.content ?? ''
    },
    set(value) {
        emit('update:content', value)
    }
})
</script>

<template>
    <v-ace-editor
        v-model:value="value"
        lang="nginx"
        theme="monokai"
        :style="{
            minHeight: defaultHeight || '100vh'
        }"/>
</template>

<style scoped>

</style>
