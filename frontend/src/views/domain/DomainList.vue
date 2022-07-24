<template>
    <a-card :title="$gettext('Manage Sites')">
        <std-table
            :api="api"
            :columns="columns"
            data_key="configs"
            :disable_search="true"
            row-key="name"
            ref="table"
            @clickEdit="r => this.$router.push({
                path: '/domain/' + r
            })"
        >
            <template #actions="{record}">
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

<script>
import StdTable from '@/components/StdDataDisplay/StdTable'
import $gettext, {$interpolate} from "@/lib/translate/gettext";

const columns = [{
    title: $gettext('Name'),
    dataIndex: 'name',
    scopedSlots: {customRender:  'name'},
    sorter: true,
    pithy: true
}, {
    title: $gettext('Status'),
    dataIndex: 'enabled',
    badge: true,
    scopedSlots: {customRender: 'enabled'},
    mask: {
        true: $gettext('Enabled'),
        false: $gettext('Disabled')
    },
    sorter: true,
    pithy: true
}, {
    title: $gettext('Updated at'),
    dataIndex: 'modify',
    datetime: true,
    scopedSlots: {customRender: 'modify'},
    sorter: true,
    pithy: true
}, {
    title: $gettext('Action'),
    dataIndex: 'action',
    scopedSlots: {customRender: 'action'}
}]

export default {
    name: 'Domain',
    components: {StdTable},
    data() {
        return {
            api: this.$api.domain,
            columns
        }
    },
    methods: {
        enable(name) {
            this.$api.domain.enable(name).then(() => {
                this.$message.success($gettext('Enabled successfully'))
                this.$refs.table.get_list()
            }).catch(r => {
                console.log(r)
                this.$message.error($interpolate($gettext('Failed to enable %{msg}'), {msg: r.message ?? ''}), 10)
            })
        },
        disable(name) {
            this.$api.domain.disable(name).then(() => {
                this.$message.success($gettext('Disabled successfully'))
                this.$refs.table.get_list()
            }).catch(r => {
                console.log(r)
                this.$message.error($interpolate($gettext('Failed to disable %{msg}'), {msg: r.message ?? ''}))
            })
        }
    }
}
</script>

<style scoped>

</style>
