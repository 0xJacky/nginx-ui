<template>
    <div class="std-curd">
        <a-card :title="title">
            <a v-if="!disable_add" slot="extra" @click="add">添加</a>
            <std-table
                ref="table"
                v-bind="this.$props"
                @clickEdit="edit"
                @selected="onSelect"
                :key="update"
            >
                <template v-slot:actions="slotProps">
                    <slot name="actions" :actions="slotProps.record"/>
                </template>
            </std-table>
        </a-card>
        <a-modal
            class="std-curd-edit-modal"
            :mask="false"
            :title="data.id ? '编辑 ID: ' + data.id : '添加'"
            :visible="visible"
            cancel-text="关闭"
            ok-text="保存"
            @cancel="visible=false;error={}"
            @ok="ok"
            :width="600"
            destroyOnClose
        >
            <std-data-entry ref="std_data_entry" :data-list="editableColumns()" :data-source="data"
                            :error="error">
                <div slot="supplement">
                    <slot name="supplement"></slot>
                </div>
                <div slot="action">
                    <slot name="action"></slot>
                </div>
            </std-data-entry>
        </a-modal>
        <footer-tool-bar v-if="batch_columns.length">
            <a-space>
                当前已选中{{ selected.length }}条数据
                <a-button :disabled="!selected.length"
                          @click="selected=[];update++">清空选中
                </a-button>
                <a-button type="primary"
                          :disabled="!selected.length"
                          @click="visible_batch_edit=true" ghost>批量修改
                </a-button>
            </a-space>
        </footer-tool-bar>
        <a-modal
            :mask="false"
            title="批量修改"
            :visible="visible_batch_edit"
            cancel-text="取消"
            ok-text="保存"
            @cancel="visible_batch_edit=false"
            @ok="okBatchEdit"
        >
            留空则不修改
            <std-data-entry :data-list="batch_columns" :data-source="data"/>
        </a-modal>
    </div>
</template>

<script>
import StdTable from './StdTable'
import StdDataEntry from '@/components/StdDataEntry/StdDataEntry'
import FooterToolBar from "@/components/FooterToolbar/FooterToolBar"

export default {
    name: 'StdCurd',
    components: {
        StdTable,
        StdDataEntry,
        FooterToolBar
    },
    props: {
        api: Object,
        columns: Array,
        title: {
            type: String,
            default: '列表'
        },
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
        },
        get_params: {
            type: Object,
            default() {
                return {}
            }
        },
        editable: {
            type: Boolean,
            default: true
        },
    },
    data() {
        return {
            visible: false,
            visible_batch_edit: false,
            data: {
                id: null,
            },
            error: {},
            params: {},
            selected: [],
            batch_columns: this.batchColumns(),
            update: 0,
        }
    },
    methods: {
        onSelect(keys) {
            this.selected = keys
        },
        batchColumns() {
            return this.columns.filter((column) => {
                return column.batch
                    && column.edit && column.edit.type !== 'upload'
                    && column.edit.type !== 'transfer'
            })
        },
        okBatchEdit() {
            this.api.batchSave(this.selected, this.data)
                .then(() => {
                    this.$message.success('批量修改成功')
                    this.$refs.table.get_list()
                }).catch(e => {
                this.$message.error(e.message)
            })
        },
        editableColumns() {
            return this.columns.filter((c) => {
                return c.edit
            })
        },
        uploadColumns() {
            return this.columns.filter(c => {
                return c.edit && c.edit.type === 'upload'
            })
        },
        async add() {
            this.data = {
                id: null
            }
            this.visible = true
        },
        async do_upload() {
            const columns = await this.uploadColumns()

            for (let i = 0; i < columns.length; i++) {
                const refs = this.$refs.std_data_entry.$refs
                const t = refs['std_upload_' + columns[i].dataIndex][0]
                if (t) {
                    await t.upload()
                }
            }
        },
        async ok() {
            this.error = {}
            if (this.data.id) {
                await this.do_upload()
                this.api.save((this.data.id ? this.data.id : null), this.data).then(r => {
                    this.$message.success('保存成功')
                    this.data = Object.assign(this.data, r)
                    this.$refs.table.get_list()
                }).catch(error => {
                    this.$message.error((error.message ? error.message : '保存失败'), 5)
                    this.error = error.errors
                })

            } else {
                this.api.save((this.data.id ? this.data.id : null), this.data).then(r => {
                    this.$message.success('保存成功')
                    this.data = this.extend(this.data, r)
                    this.$nextTick().then(() => {
                        this.do_upload()
                    })
                    this.$refs.table.get_list()
                }).catch(error => {
                    this.$message.error((error.message ? error.message : '保存失败'), 5)
                    this.error = error.errors
                })
            }
        },
        edit(id) {
            this.api.get(id).then(r => {
                this.data = r
                this.visible = true
            }).catch(e => {
                console.log(e)
                this.$message.error('系统错误')
            })
        }
    }
}
</script>

<style lang="less" scoped>

</style>
