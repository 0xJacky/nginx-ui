<script setup lang="tsx">
import StdTable from '@/components/StdDataDisplay/StdTable.vue'

import {customRender, datetime} from '@/components/StdDataDisplay/StdTableTransformer'
import {useGettext} from 'vue3-gettext'

const {$gettext, interpolate} = useGettext()

import domain from '@/api/domain'
import {Badge, message} from 'ant-design-vue'
import {h, ref} from 'vue'
import {input} from '@/components/StdDataEntry'
import SiteDuplicate from '@/views/domain/SiteDuplicate.vue'

const columns = [{
    title: () => $gettext('Name'),
    dataIndex: 'name',
    sorter: true,
    pithy: true,
    edit: {
        type: input
    },
    search: true
}, {
    title: () => $gettext('Status'),
    dataIndex: 'enabled',
    customRender: (args: customRender) => {
        const template: any = []
        const {text} = args
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
    title: () => $gettext('Updated at'),
    dataIndex: 'modify',
    customRender: datetime,
    sorter: true,
    pithy: true
}, {
    title: () => $gettext('Action'),
    dataIndex: 'action'
}]

const table = ref(null)

interface Table {
    get_list(): void
}

function enable(name: any) {
    domain.enable(name).then(() => {
        message.success($gettext('Enabled successfully'))
        const t: Table | null = table.value
        t!.get_list()
    }).catch(r => {
        message.error(interpolate($gettext('Failed to enable %{msg}'), {msg: r.message ?? ''}), 10)
    })
}

function disable(name: any) {
    domain.disable(name).then(() => {
        message.success($gettext('Disabled successfully'))
        const t: Table | null = table.value
        t!.get_list()
    }).catch(r => {
        message.error(interpolate($gettext('Failed to disable %{msg}'), {msg: r.message ?? ''}))
    })
}

function destroy(site_name: any) {
    domain.destroy(site_name).then(() => {
        const t: Table | null = table.value
        t!.get_list()
        message.success(interpolate($gettext('Delete site: %{site_name}'), {site_name: site_name}))
    }).catch((e: any) => {
        message.error(e?.message ?? $gettext('Server error'))
    })
}

const show_duplicator = ref(false)

const target = ref('')

function handle_click_duplicate(name: string) {
    show_duplicator.value = true
    target.value = name
}
</script>

<template>
    <a-card :title="$gettext('Manage Sites')">
        <std-table
            :api="domain"
            :columns="columns"
            row-key="name"
            ref="table"
            @clickEdit="r => $router.push({
                path: '/domain/' + r
            })"
            :deletable="false"
        >
            <template #actions="{record}">
                <a-divider type="vertical"/>
                <a-button type="link" size="small" v-if="record.enabled" @click="disable(record.name)">
                    {{ $gettext('Disabled') }}
                </a-button>
                <a-button type="link" size="small" v-else @click="enable(record.name)">
                    {{ $gettext('Enabled') }}
                </a-button>
                <a-divider type="vertical"/>
                <a-button type="link" size="small" @click="handle_click_duplicate(record.name)">
                    {{ $gettext('Duplicate') }}
                </a-button>
                <a-divider type="vertical"/>
                <a-popconfirm
                    :cancelText="$gettext('No')"
                    :okText="$gettext('OK')"
                    :title="$gettext('Are you sure you want to delete?')"
                    @confirm="destroy(record['name'])">
                    <a-button type="link" size="small" :disabled="record.enabled">
                        {{ $gettext('Delete') }}
                    </a-button>
                </a-popconfirm>
            </template>
        </std-table>
        <site-duplicate v-model:visible="show_duplicator" :name="target" @duplicated="table.get_list()"/>
    </a-card>
</template>

<style scoped>

</style>
