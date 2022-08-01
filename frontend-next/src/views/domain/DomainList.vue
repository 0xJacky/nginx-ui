<script setup lang="tsx">
import StdTable from '@/components/StdDataDisplay/StdTable.vue'

import {badge, customRender, datetime} from '@/components/StdDataDisplay/StdTableTransformer'
import {useGettext} from 'vue3-gettext'

const {$gettext, interpolate} = useGettext()

import domain from '@/api/domain'
import {Badge, message} from 'ant-design-vue'
import {h, ref} from 'vue'

const columns = [{
    title: () => $gettext('Name'),
    dataIndex: 'name',
    sorter: true,
    pithy: true
}, {
    title: () => $gettext('Status'),
    dataIndex: 'enabled',
    customRender: (args: customRender) => {
        const template: any = []
        const {text, column} = args
        if (text === true || text > 0) {
            template.push(<Badge status="success"/>)
            template.push($gettext('Enabled'))
        } else {
            template.push(<Badge status="error"/>)
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
    dataIndex: 'action',
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
</script>

<template>
    <a-card :title="$gettext('Manage Sites')">
        <std-table
            :api="domain"
            :columns="columns"
            :disable_search="true"
            row-key="name"
            ref="table"
            @clickEdit="r => this.$router.push({
                path: '/domain/' + r
            })"
            :deletable="false"
        >
            <template #actions="{record}">
                <template v-if="!record.enabled">
                    <a-divider type="vertical"/>
                    <a-popconfirm
                        :cancelText="$gettext('No')"
                        :okText="$gettext('OK')"
                        :title="$gettext('Are you sure you want to delete ?')"
                        @confirm="destroy(record['name'])">
                        <a v-translate>Delete</a>
                    </a-popconfirm>
                </template>
                <a-divider type="vertical"/>
                <a v-if="record.enabled" @click="disable(record.name)">
                    {{ $gettext('Disabled') }}
                </a>
                <a v-else @click="enable(record.name)">
                    {{ $gettext('Enabled') }}
                </a>
            </template>
        </std-table>
    </a-card>
</template>

<style scoped>

</style>
