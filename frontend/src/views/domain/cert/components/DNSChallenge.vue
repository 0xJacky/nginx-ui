<script setup lang="ts">
import {computed, inject, ref, watch} from 'vue'
import auto_cert from '@/api/auto_cert'
import {useGettext} from 'vue3-gettext'
import {SelectProps} from 'ant-design-vue'
import dns_credential from '@/api/dns_credential'

const {$gettext} = useGettext()
const providers: any = ref([])
const credentials: any = ref([])

const data: any = inject('data')!

const code = computed(() => {
    return data.code
})

function init() {
    providers.value?.forEach((v: any, k: number) => {
        if (v.code === code.value) {
            provider_idx.value = k
        }
    })
}

auto_cert.get_dns_providers().then(r => {
    providers.value = r
}).then(() => {
    init()
})

const provider_idx = ref()

const current: any = computed(() => {
    return providers.value?.[provider_idx.value]
})


watch(code, init)

watch(current, () => {
    credentials.value = []
    data.code = current.value.code
    data.provider = current.value.name
    data.dns_credential_id = null

    dns_credential.get_list({provider: data.provider}).then(r => {
        r.data.forEach((v: any) => {
            credentials.value.push({
                value: v.id,
                label: v.name
            })
        })

    })
})

const options = computed<SelectProps['options']>(() => {
    let list: SelectProps['options'] = []

    providers.value.forEach((v: any, k: number) => {
        list!.push({
            value: k,
            label: v.name
        })
    })

    return list
})

const filterOption = (input: string, option: any) => {
    return option.label.toLowerCase().indexOf(input.toLowerCase()) >= 0
}
</script>

<template>
    <a-form layout="vertical">
        <a-form-item :label="$gettext('DNS Provider')">
            <a-select v-model:value="provider_idx" show-search :options="options" :filter-option="filterOption"/>
        </a-form-item>
        <a-form-item v-if="provider_idx>-1" :label="$gettext('Credential')" :rules="[{ required: true }]">
            <a-select :options="credentials" v-model:value="data.dns_credential_id"/>
        </a-form-item>
    </a-form>
</template>

<style lang="less" scoped>

</style>
