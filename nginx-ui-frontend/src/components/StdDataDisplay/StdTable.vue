<template>
    <div class="std-table">
        <a-form v-if="!disable_search" layout="inline">
            <a-form-item
                v-for="c in searchColumns(columns)" :key="c.dataIndex?c.dataIndex:c.name"
                :label="c.title">
                <a-input v-if="c.search.type==='input'" v-model="params[c.dataIndex]"/>
                <a-checkbox
                    v-if="c.search.type==='checkbox'"
                    :default-checked="c.search.default"
                    :name="c.search.condition?c.search.condition:c.dataIndex"
                    @change="checked"/>
                <a-slider
                    v-else-if="c.search.type==='slider'"
                    v-model="params[c.dataIndex]"
                    :marks="c.mask"
                    :max="c.search.max"
                    :min="c.search.min"
                    style="width: 130px"
                />
                <a-select v-if="c.search.type==='select'" v-model="params[c.dataIndex]"
                          style="width: 130px">
                    <a-select-option v-for="(v,k) in c.mask" :key="k" :value="k">{{ v }}</a-select-option>
                </a-select>
            </a-form-item>
            <a-form-item :wrapper-col="{span:8}">
                <a-button type="primary" @click="get_list()">查询</a-button>
            </a-form-item>
            <a-form-item :wrapper-col="{span:8}">
                <a-button @click="reset_search">重置</a-button>
            </a-form-item>
        </a-form>
        <div v-if="soft_delete" style="text-align: right">
            <a v-if="params['trashed']" href="javascript:;"
               @click="params['trashed']=false; get_list()">返回</a>
            <a v-else href="javascript:;" @click="params['trashed']=true; get_list()">回收站</a>
        </div>
        <a-table
            :columns="pithyColumns(columns)"
            :customRow="row"
            :data-source="data_source"
            :loading="loading"
            :pagination="false"
            :row-key="'name'"
            :rowSelection="{
       selectedRowKeys: selectedRowKeys,
       onChange: onSelectChange,
       onSelect: onSelect,
       type: selectionType,
       }"
            @change="stdChange"
        >
            <template
                v-for="c in pithyColumns(columns)"
                :slot="c.scopedSlots.customRender"
                slot-scope="text, record">
      <span v-if="c.badge" :key="c.dataIndex">
        <a-badge v-if="text === true || text > 0" status="success"/>
        <a-badge v-else status="error"/>
        {{ c.mask ? c.mask[text] : text }}
      </span>
                <span v-else-if="c.datetime" :key="c.dataIndex">{{ moment(text).format("yyyy-MM-DD HH:mm:ss") }}</span>
                <span v-else-if="c.click" :key="c.dataIndex">
          <a href="javascript:;" @click="handleClick(record[c.click.index?c.click.index:c.dataIndex],
          c.click.index?c.click.index:c.dataIndex,
          c.click.method, c.click.path)">
            {{ text != null ? text : c.default }}
          </a>
        </span>
                <span v-else :key="c.dataIndex">
        {{ text != null ? (c.mask ? c.mask[text] : text) : c.default }}
      </span>
            </template>
            <span v-if="!pithy" slot="action" slot-scope="text, record">
                    <slot name="action" :record="record" />
                    <a href="javascript:;" @click="$emit('clickEdit', record)">
                        <template v-if="edit_text">{{ edit_text }}</template>
                        <template v-else>编辑</template>
                    </a>
                <template v-if="deletable">
                    <a-divider type="vertical"/>
                      <a-popconfirm
                          v-if="soft_delete&&params.trashed"
                          cancelText="再想想"
                          okText="是的" title="你确定要反删除?"
                          @confirm="restore(record.name)">
                        <a href="javascript:;">反删除</a>
                      </a-popconfirm>
                      <a-popconfirm
                          v-else
                          cancelText="再想想"
                          okText="是的" title="你确定要删除?"
                          @confirm="destroy(record.name)"
                      >
                        <a href="javascript:;">删除</a>
                      </a-popconfirm>
                </template>

                </span>

        </a-table>
        <std-pagination :pagination="pagination" @changePage="get_list"/>
    </div>
</template>

<script>
import StdPagination from './StdPagination'
import moment from "moment"

export default {
    name: 'StdTable',
    components: {
        StdPagination,
    },
    props: {
        columns: Array,
        api: Object,
        data_key: String,
        selectionType: {
            type: String,
            default: 'checkbox',
            validator: function (value) {
                return ['checkbox', 'radio'].indexOf(value) !== -1
            }
        },
        pithy: {
            type: Boolean,
            default: false
        },
        disable_search: {
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
            moment,
            data_source: [],
            loading: true,
            pagination: {
                total: 1,
                per_page: 10,
                current_page: 1,
                total_pages: 1
            },
            params: {},
            selectedRowKeys: [],
        }
    },
    mounted() {
        this.get_list()
    },
    methods: {
        get_list(page_num = null) {
            this.loading = true
            this.params['page'] = page_num
            this.api.get_list(this.params).then(response => {
                if (response[this.data_key] === undefined && response.data !== undefined) {
                    this.data_source = response.data
                } else {
                    this.data_source = response[this.data_key]
                }
                if (response.pagination !== undefined) {
                    this.pagination = response.pagination
                }
                this.loading = false
            }).catch(e => {
                console.log(e)
                this.$message.error('服务器错误')
            })
        },
        stdChange(pagination, filters, sorter) {
            if (sorter) {
                this.params['order_by'] = sorter.field
                this.params['sort'] = sorter.order === 'ascend' ? 'asc' : 'desc'
                this.$nextTick(() => {
                    this.get_list()
                })
            }
        },
        destroy(id) {
            this.api.destroy(id).then(() => {
                this.get_list()
                this.$message.success('删除 ID: ' + id + ' 成功')
            }).catch(e => {
                this.$message.error('服务器错误' + (e.message ? " " + e.message : ""))
            })
        },
        restore(id) {
            this.api.restore(id).then(() => {
                this.get_list()
                this.$message.success('反删除 ID: ' + id + ' 成功')
            }).catch(() => {
                this.$message.error('服务器错误')
            })
        },
        searchColumns(columns) {
            return columns.filter((column) => {
                return column.search
            })
        },
        pithyColumns(columns) {
            if (this.pithy) {
                return columns.filter((c) => {
                    return c.pithy === true && c.display !== false
                })
            }
            return columns.filter((c) => {
                return c.display !== false
            })
        },
        checked(c) {
            this.params[c.target.value] = c.target.checked
        },
        onSelectChange(selectedRowKeys) {
            this.selectedRowKeys = selectedRowKeys
            this.$emit('selected', selectedRowKeys)
        },
        onSelect(record) {
            console.log(record)
            this.$emit('selectedRecord', record)
        },
        handleClick(data, index, method = '', path = '') {
            if (method === 'router') {
                this.$router.push(path + '/' + data).then()
            } else {
                this.params[index] = data
                this.get_list()
            }
        },
        row(record) {
            return {
                on: {
                    click: () => {
                        this.$emit('clickRow', record.id)
                    }
                }
            }
        },
        async reset_search() {
            this.params = {}
            await this.get_list()
        }
    }
}
</script>

<style lang="less" scoped>
.ant-form {
    margin: 10px 0 20px 0;
}

.ant-slider {
    min-width: 90px;
}

.std-table {
    .ant-table-wrapper {
        overflow: scroll;
    }
}
</style>
