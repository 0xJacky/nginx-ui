<script setup lang="ts">
import ngx from '@/api/ngx'
import {useGettext} from 'vue3-gettext'
import {ref} from 'vue'
import logLevel from '@/views/config/constants'
import Template from '@/views/template/Template.vue'

const {$gettext} = useGettext()

const data = ref({
    level: 0,
    message: ''
})

test()

function test() {
    ngx.test().then(r => {
        data.value = r
    })
}

defineExpose({
    test
})
</script>

<template>
    <a-card class="inspect-container" :title="$gettext('Inspect Configurations')">
        <a-alert :message="$gettext('Configuration file is test successful')" type="success"
                 show-icon v-if="data?.level<logLevel.Debug"/>
        <a-alert
            :message="$gettext('Warning')"
            type="warning"
            show-icon
            v-else-if="data?.level===logLevel.Warn"
        >
            <template #description>
                {{ data.message }}
            </template>
        </a-alert>

        <a-alert
            :message="$gettext('Error')"
            type="error"
            show-icon
            v-else-if="data?.level>logLevel.Warn"
        >
            <template #description>
                {{ data.message }}
            </template>
        </a-alert>
    </a-card>
</template>

<style lang="less" scoped>
.inspect-container {
    margin-bottom: 20px;
}

:deep(.ant-alert-description) {
    white-space: pre-line;
}
</style>
