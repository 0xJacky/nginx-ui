<script setup lang="tsx">
import {useGettext} from 'vue3-gettext'
import {input} from '@/components/StdDataEntry'
import {customRender, datetime} from '@/components/StdDataDisplay/StdTableTransformer'
import {h} from 'vue'
import {Badge} from 'ant-design-vue'
import cert from '@/api/cert'
import StdCurd from '@/components/StdDataDisplay/StdCurd.vue'
import Template from '@/views/template/Template.vue'
import CodeEditor from '@/components/CodeEditor/CodeEditor.vue'
import CertInfo from '@/views/domain/cert/CertInfo.vue'

const {$gettext} = useGettext()

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
    title: () => $gettext('Domain'),
    dataIndex: 'domain',
    sorter: true,
    pithy: true,
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
}, {
    title: () => $gettext('SSL Certificate Path'),
    dataIndex: 'ssl_certificate_path',
    edit: {
        type: input
    },
    display: false
}, {
    title: () => $gettext('SSL Certificate Key Path'),
    dataIndex: 'ssl_certificate_key_path',
    edit: {
        type: input
    },
    display: false
}, {
    title: () => $gettext('Updated at'),
    dataIndex: 'updated_at',
    customRender: datetime,
    sorter: true,
    pithy: true
}, {
    title: () => $gettext('Action'),
    dataIndex: 'action'
}]
</script>

<template>
    <std-curd :title="$gettext('Certification')" :api="cert" :columns="columns"
              row-key="name"
    >
        <template #beforeEdit="{data}">
            <div v-if="data.auto_cert===1" style="margin-bottom: 15px">
                <a-alert
                    :message="$gettext('Auto cert is enabled, please do not modify this certification.')" type="info"
                    show-icon/>
            </div>
            <a-form layout="vertical" v-if="data.certificate_info">
                <a-form-item :label="$gettext('Certificate Status')">
                    <cert-info :cert="data.certificate_info"/>
                </a-form-item>
            </a-form>
        </template>
        <template #edit="{data}">
            <a-form layout="vertical">
                <a-form-item :label="$gettext('SSL Certification Content')">
                    <code-editor v-model:content="data.ssl_certification" default-height="200px"/>
                </a-form-item>
                <a-form-item :label="$gettext('SSL Certification Key Content')">
                    <code-editor v-model:content="data.ssl_certification_key" default-height="200px"/>
                </a-form-item>
            </a-form>
        </template>
    </std-curd>
</template>

<style lang="less" scoped>

</style>
