<template>
    <div>
        <a-card :title="title">
            <a v-if="!disable_add" slot="extra" @click="add">添加</a>
            <std-table
                ref="table"
                :api="api"
                :columns="columns"
                :data_key="data_key"
                :deletable="deletable"
                :disable_search="disable_search"
                :edit_text="edit_text"
                :soft_delete="soft_delete"
                @clickEdit="edit"
            />
        </a-card>
        <a-modal
            :mask="false"
            :title="data.id ? '编辑 ID: ' + data.id : '添加'"
            :visible="visible"
            cancel-text="取消"
            ok-text="保存"
            @cancel="visible=false;error={}"
            @ok="ok"
        >
            <std-data-entry ref="std_data_entry" :data-list="editableColumns(columns)" :data-source="data"
                            :error="error"/>
        </a-modal>
    </div>
</template>

<script>
import StdTable from './StdTable'
import StdDataEntry from '@/components/StdDataEntry/StdDataEntry'

export default {
    name: 'StdCurd',
    components: {
        StdTable,
        StdDataEntry
    },
    props: {
        title: {
            type: String,
            default: '列表'
        },
        api: Object,
        columns: Array,
        data_key: {
            type: String,
            default: 'data'
        },
        disable_search: {
            type: Boolean,
            default: false
        },
        disable_add: {
            type: Boolean,
            default: false
        },
        soft_delete: {
            type: Boolean,
            default: false
        },
        edit_text: String,
        deletable: {
            type: Boolean,
            default: true
        }
    },
    data() {
        return {
            visible: false,
            data: {
                id: null,
            },
            error: {}
        }
    },
    methods: {
        editableColumns(columns) {
            return columns.filter((c) => {
                return c.edit
            })
        },
        uploadColumns(columns) {
            return columns.filter((c) => {
                return c.edit && c.edit.type === 'upload'
            })
        },
        add() {
            this.visible = true
            this.data = {
                id: null
            }
        },
        ok() {
            this.api.save((this.data.id ? this.data.id : null), this.data).then(r => {
                this.$message.success('保存成功')
                const refs = this.$refs.std_data_entry.$refs
                this.uploadColumns(this.columns).forEach(c => {
                    const t = refs['std_upload_' + c.dataIndex][0]
                    if (t) {
                        t.upload()
                    }
                    delete r[c.dataIndex]
                })
                this.data = this.extend(this.data, r)
                this.$refs.table.get_list()
            }).catch(error => {
                this.$message.error('保存失败')
                this.error = error.errors
            })
        },
        edit(id) {
            this.api.get(id).then(r => {
                this.data = r
                this.visible = true
            }).catch(() => {
                this.$message.error('服务器错误')
            })
        }
    }
}
</script>

<style scoped>

</style>
