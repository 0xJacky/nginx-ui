<script setup lang="tsx">
import {useGettext} from 'vue3-gettext'
import {datetime} from '@/components/StdDataDisplay/StdTableTransformer'
import dns_credential from '@/api/dns_credential'
import StdCurd from '@/components/StdDataDisplay/StdCurd.vue'
import Template from '@/views/template/Template.vue'
import DNSChallenge from '@/views/domain/cert/components/DNSChallenge.vue'
import {input} from '@/components/StdDataEntry'

const {$gettext, interpolate} = useGettext()

const columns = [{
    title: () => $gettext('Name'),
    dataIndex: 'name',
    sorter: true,
    pithy: true,
    edit: {
        type: input
    }
}, {
    title: () => $gettext('Provider'),
    dataIndex: ['config', 'name'],
    sorter: true,
    pithy: true
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
    <std-curd :title="$gettext('DNS Credentials')" :api="dns_credential" :columns="columns"
              row-key="name"
    >
        <template #beforeEdit>
            <a-alert type="info" show-icon :message="$gettext('Note')">
                <template #description>
                    <p v-translate>
                        Please fill in the API authentication credentials provided by your DNS provider.
                        We will add one or more TXT records to the DNS records of your domain for ownership
                        verification.
                        Once the verification is complete, the records will be removed.
                        Please note that the time configurations below are all in seconds.
                    </p>
                </template>
            </a-alert>
        </template>
        <template #edit="{data}">
            <d-n-s-challenge/>
        </template>
    </std-curd>
</template>

<style lang="less" scoped>

</style>
