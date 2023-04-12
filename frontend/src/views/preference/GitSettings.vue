<script setup lang="ts">
import {useGettext} from 'vue3-gettext'
import {inject} from 'vue'
import {IData} from '@/views/preference/typedef'

const {$gettext} = useGettext()

const data: IData = inject('data')!
</script>

<template>
    <a-form layout="vertical">
        <a-form-item :label="$gettext('Repo url')">
            <a-input v-model:value="data.git.url"/>
        </a-form-item>
        <a-form-item :label="$gettext('Username')">
            <a-input v-model:value="data.git.username"/>
        </a-form-item>
    </a-form>
</template>

<style lang="less" scoped>

</style>
