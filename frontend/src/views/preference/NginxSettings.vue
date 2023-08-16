<script setup lang="ts">
import {useGettext} from 'vue3-gettext'
import {inject} from 'vue'
import {IData} from '@/views/preference/typedef'

const {$gettext} = useGettext()

const data: IData = inject('data')!
</script>

<template>
    <a-form layout="vertical">
        <a-form-item :label="$gettext('Nginx Access Log Path')">
            <a-input v-model:value="data.nginx.access_log_path"/>
        </a-form-item>
        <a-form-item :label="$gettext('Nginx Error Log Path')">
            <a-input v-model:value="data.nginx.error_log_path"/>
        </a-form-item>
    </a-form>
</template>

<style lang="less" scoped>

</style>
