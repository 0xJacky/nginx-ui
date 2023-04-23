<script setup lang="tsx">
import {useGettext} from 'vue3-gettext'
import {ComputedRef, h, inject, ref} from 'vue'
import StdTable from '@/components/StdDataDisplay/StdTable.vue'
import cert from '@/api/cert'
import {customRender} from '@/components/StdDataDisplay/StdTableTransformer'
import {input} from '@/components/StdDataEntry'
import {Badge} from 'ant-design-vue'

const {$gettext} = useGettext()

const props = defineProps(['directivesMap'])

const current_server_directives: ComputedRef<any> | undefined = inject('current_server_directives')

const visible = ref(false)

const record: any = ref({})

const columns = [{
    title: () => $gettext('Name'),
    dataIndex: 'name',
    sorter: true,
    pithy: true,
    customRender: (args: customRender) => {
        const {text, record} = args
        if (!text) {
            return h('div', record.domain)
        }
        return h('div', text)
    },
    edit: {
        type: input
    },
    search: true
}, {
    title: () => $gettext('Auto Cert'),
    dataIndex: 'auto_cert',
    customRender: (args: customRender) => {
        const template: any = []
        const {text, column} = args
        if (text === true || text > 0) {
            template.push(<Badge status="success"/>)
            template.push($gettext('Enabled'))
        } else {
            template.push(<Badge status="warning"/>)
            template.push($gettext('Disabled'))
        }
        return h('div', template)
    },
    sorter: true,
    pithy: true
}]

function open() {
    visible.value = true
}

function onSelectedRecord(r: any) {
    record.value = r
}

function ok() {
    if (props.directivesMap['ssl_certificate']?.[0]) {
        props.directivesMap['ssl_certificate'][0]['params'] = record.value.ssl_certificate_path
    } else {
        current_server_directives?.value.push({
            directive: 'ssl_certificate',
            params: record.value.ssl_certificate_path
        })
    }
    if (props.directivesMap['ssl_certificate_key']?.[0]) {
        props.directivesMap['ssl_certificate_key'][0]['params'] = record.value.ssl_certificate_key_path
    } else {
        current_server_directives?.value.push({
            directive: 'ssl_certificate_key',
            params: record.value.ssl_certificate_key_path
        })
    }
    visible.value = false
}
</script>

<template>
    <div>
        <a-button @click="open">{{ $gettext('Change Certificate') }}</a-button>
        <a-modal
            :title="$gettext('Change Certificate')"
            v-model:visible="visible"
            :mask="false"
            @ok="ok"
        >
            <std-table
                :api="cert"
                :pithy="true"
                :columns="columns"
                selectionType="radio"
                @onSelectedRecord="onSelectedRecord"
            />
        </a-modal>
    </div>
</template>

<style lang="less" scoped>

</style>
