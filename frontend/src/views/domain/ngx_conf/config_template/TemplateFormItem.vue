<script setup lang="ts">
import {computed, inject, watch} from 'vue'
import {storeToRefs} from 'pinia'
import {useSettingsStore} from '@/pinia'
import {useGettext} from 'vue3-gettext'
import _ from 'lodash'

const {$gettext} = useGettext()

const {language} = storeToRefs(useSettingsStore())
const props = defineProps(['data', 'name'])

const trans_name = computed(() => {
    return props.data?.name?.[language.value] ?? props.data?.name?.en ?? ''
})

const build_template: any = inject('build_template')!

const value = computed(() => props.data.value)

watch(value, _.throttle(build_template, 500))
</script>

<template>
    <a-form-item :label="trans_name">
        <a-input v-if="data.type === 'string'" v-model:value="data.value"/>
        <a-switch v-else-if="data.type === 'boolean'" v-model:checked="data.value"/>
    </a-form-item>
</template>

<style lang="less" scoped>

</style>
