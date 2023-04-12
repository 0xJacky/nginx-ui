<script setup lang="ts">
import {computed, inject, Ref, ref, watch} from 'vue'
import auto_cert from '@/api/auto_cert'
import {useGettext} from 'vue3-gettext'
import {SelectProps} from 'ant-design-vue'

const {$gettext} = useGettext()
const providers: any = ref([])

const data: any = inject('data')!

auto_cert.get_dns_providers().then(r => {
    providers.value = r
})

const provider_idx = ref()

const current: any = computed(() => {
    return providers.value?.[provider_idx.value]
})

watch(current, () => {
    data.code = current.value.code
    auto_cert.get_dns_provider(current.value.code).then(r => {
        Object.assign(current.value, r)
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
        <template v-if="current?.configuration?.credentials">
            <h4>{{ $gettext('Credentials') }}</h4>
            <a-form-item :label="k" v-for="(v,k) in current?.configuration?.credentials"
                         :extra="v" :rules="[{ required: true }]">
                <a-input v-model:value="data.configuration.credentials[k]"/>
            </a-form-item>
        </template>
        <template v-if="current?.configuration?.additional">
            <h4>{{ $gettext('Additional') }}</h4>
            <a-form-item :label="k" v-for="(v,k) in current?.configuration?.additional" :extra="v">
                <a-input v-model:value="data.configuration.additional[k]"/>
            </a-form-item>
        </template>
    </a-form>
</template>

<style lang="less" scoped>

</style>
