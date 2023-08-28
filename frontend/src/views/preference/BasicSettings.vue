<script setup lang="ts">
import {useGettext} from 'vue3-gettext'
import {inject} from 'vue'
import {IData} from '@/views/preference/typedef'

const {$gettext} = useGettext()
const data: IData = inject('data') as IData
const theme = inject('theme')
</script>

<template>
    <a-form layout="vertical">
        <a-form-item :label="$gettext('Theme')">
            <a-select v-model:value="theme">
                <a-select-option value="auto">
                    {{ $gettext('Auto') }}
                </a-select-option>
                <a-select-option value="light">
                    {{ $gettext('Light') }}
                </a-select-option>
                <a-select-option value="dark">
                    {{ $gettext('Dark') }}
                </a-select-option>
            </a-select>
        </a-form-item>
        <a-form-item :label="$gettext('HTTP Host')">
            <p>{{ data.server.http_host }}</p>
        </a-form-item>
        <a-form-item :label="$gettext('HTTP Port')">
            <p>{{ data.server.http_port }}</p>
        </a-form-item>
        <a-form-item :label="$gettext('Run Mode')">
            <p>{{ data.server.run_mode }}</p>
        </a-form-item>
        <a-form-item :label="$gettext('Jwt Secret')">
            <p>{{ data.server.jwt_secret }}</p>
        </a-form-item>
        <a-form-item :label="$gettext('Node Secret')">
            <p>{{ data.server.node_secret }}</p>
        </a-form-item>
        <a-form-item :label="$gettext('Terminal Start Command')">
            <p>{{ data.server.start_cmd }}</p>
        </a-form-item>
        <a-form-item :label="$gettext('HTTP Challenge Port')">
            <a-input-number v-model:value="data.server.http_challenge_port"/>
        </a-form-item>
        <a-form-item :label="$gettext('Github Proxy')">
            <a-input v-model:value="data.server.github_proxy"
                     :placeholder="$gettext('Chinese user: https://ghproxy.com/')"/>
        </a-form-item>
        <a-form-item :label="$gettext('CADir')">
            <a-input v-model:value="data.server.ca_dir"/>
        </a-form-item>
    </a-form>
</template>

<style lang="less" scoped>

</style>
