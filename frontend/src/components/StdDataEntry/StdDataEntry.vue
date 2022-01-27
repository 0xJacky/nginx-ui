<template>
    <a-form :layout="layout" class="std-data-entry">
        <a-form-item
            v-for="d in M_dataList" :key="d.dataIndex" :help="error[d.dataIndex] ? error[d.dataIndex].toString() : null"
            :label="d.title"
            :labelCol="d.edit.labelCol"
            :validate-status="error[d.dataIndex] ? 'error' :'success'"
            :wrapperCol="d.edit.wrapperCol"
        >
            <p v-if="d.description" v-html="d.description+'<br/>'"/>
            <a-input
                v-if="d.edit.type==='input'"
                v-model="dataSource[d.dataIndex]"
                :placeholder="getInputPlaceholder(d, dataSource)"
            />
            <a-textarea v-else-if="d.edit.type==='textarea'" v-model="dataSource[d.dataIndex]"
                        :rows="d.edit.row?d.edit.row:5"/>
            <std-select-option
                v-else-if="d.edit.type==='select'"
                v-model="temp[d.dataIndex]"
                :options="d.mask"
                :key-type="d.edit.key_type ? d.edit.key_type : 'int'"
                style="min-width: 120px"
            />

            <std-check-tag
                v-else-if="d.edit.type==='check-tag'"
                v-model="temp[d.dataIndex]"
                :options="d.mask"
            />

            <std-multi-check-tag
                v-else-if="d.edit.type==='multi-check-tag'"
                v-model="temp[d.dataIndex]"
                :data-object="temp"
                :options="d.mask"
            />

            <std-selector
                v-else-if="d.edit.type==='selector'" v-model="temp[d.dataIndex]" :api="d.edit.api"
                :columns="d.edit.columns"
                :data_key="d.edit.data_key"
                :disable_search="d.edit.disable_search" :pagination_method="d.edit.pagination_method"
                :record-value-index="d.edit.recordValueIndex" :value="fn(temp, d.edit.valueIndex)"
                :get_params="get_params_fn(d)"
                :description="d.edit.description"
                selection-type="radio"
            />

            <a-input-number v-else-if="d.edit.type==='number'" v-model="temp[d.dataIndex]"
                            :min="d.edit.min" :step="d.edit.step" :max="d.edit.max"
            />

            <std-upload v-else-if="d.edit.type==='upload'" :id="temp.id?temp.id:null" :ref="'std_upload_'+d.dataIndex"
                        v-model="temp[d.dataIndex]" :api="d.edit.api"
                        :api_delete="d.edit.api_delete"
                        :list="temp[d.dataIndex]"
                        :crop="d.edit.crop"
                        :auto-upload="d.edit.auto_upload"
                        :crop-options="d.edit.cropOptions" :type="d.edit.upload_type ? d.edit.upload_type : 'img'"
                        @uploaded="url => {$emit('uploaded', url)}"
            />

            <std-date-picker v-else-if="d.edit.type==='date_picker'" v-model="temp[d.dataIndex]"
                             :show-time="d.edit.showTime"/>

            <a-slider
                v-else-if="d.edit.type==='slider'"
                v-model="temp[d.dataIndex]"
                :marks="d.mask"
                :max="d.edit.max"
                :min="d.edit.min"
            />

            <a-switch
                v-else-if="d.edit.type==='switch'"
                v-model="temp[d.dataIndex]"
            />

            <a-checkbox
                v-else-if="d.edit.type==='checkbox'"
                v-model="temp[d.dataIndex]"
            >
                {{ d.text }}
            </a-checkbox>

            <std-check-group
                v-else-if="d.edit.type==='check-group'"
                v-model="temp[d.dataIndex]"
                :options="d.options"
                :allow-other="d.edit.allow_other"
            />

            <std-radio-group
                v-else-if="d.edit.type==='radio-group'"
                v-model="temp[d.dataIndex]"
                :options="d.options"
                :key-type="d.edit.key_type"
            />

            <std-transfer
                v-else-if="d.edit.type==='transfer'"
                v-model="temp[d.dataIndex]"
                :api="d.edit.api"
                :data-key="d.edit.dataKey"
            />

            <rich-text-editor v-else-if="d.edit.type==='rich-text'" v-model="temp[d.dataIndex]"/>

            <p v-else-if="d.edit.type==='readonly'">
                {{ d.mask ? d.mask[fn(temp, d.dataIndex)] : fn(temp, d.dataIndex) }}
            </p>

            <p v-else>{{ 'edit.type 参数非法 ' + d.edit.type }}</p>

            <p v-if="!dataSource[d.dataIndex] && d.empty_description" v-html="d.empty_description"/>
        </a-form-item>
        <a-form-item>
            <slot name="supplement"/>
            <slot name="action"/>
        </a-form-item>
    </a-form>
</template>

<script>
import StdSelectOption from './StdSelectOption'
import StdSelector from './StdSelector'
import StdUpload from './StdUpload'
import StdDatePicker from './StdDatePicker'
import StdTransfer from './StdTransfer'
import RichTextEditor from '@/components/RichText/RichTextEditor'
import StdCheckTag from '@/components/StdDataEntry/StdCheckTag'
import StdMultiCheckTag from '@/components/StdDataEntry/StdMultiCheckTag'
import StdCheckGroup from '@/components/StdDataEntry/StdCheckGroup'
import StdRadioGroup from '@/components/StdDataEntry/StdRadioGroup'

export default {
    name: 'StdDataEntry',
    components: {
        StdRadioGroup,
        StdCheckGroup,
        StdMultiCheckTag,
        StdCheckTag,
        RichTextEditor,
        StdTransfer,
        StdDatePicker,
        StdSelectOption,
        StdSelector,
        StdUpload
    },
    props: {
        dataList: [Array, Object],
        dataSource: Object,
        error: {
            type: Object,
            default() {
                return {}
            }
        },
        layout: {
            default: 'vertical',
            validator: value => {
                return ['horizontal', 'vertical', 'inline'].indexOf(value) !== -1
            }
        }
    },
    model: {
        prop: 'dataSource',
        event: 'changeDataSource'
    },
    data() {
        return {
            temp: null,
            i: 0,
            M_dataList: {}
        }
    },
    watch: {
        dataSource() {
            this.temp = this.dataSource ?? []
        },
        dataList() {
            this.M_dataList = this.editableColumns(this.dataList ?? [])
        }
    },
    created() {
        this.temp = this.dataSource ?? []
        if (this.layout === 'horizontal') {
            this.labelCol = {span: 4}
            this.wrapperCol = {span: 18}
        }
        this.M_dataList = this.editableColumns(this.dataList)
    },
    methods: {
        get_params_fn(d) {
            return {...d.edit.get_params, ...this.bindModel(d.edit.bind, this.temp)}
        },
        fn: (obj, desc) => {
            const arr = desc.split('.')
            while (arr.length) {
                const top = obj[arr.shift()]
                if (top === undefined) {
                    return null
                }
                obj = top
            }
            return obj
        },
        editableColumns(columns) {
            if (typeof columns === 'object') {
                columns = Object.values(columns)
            }
            return columns.filter((c) => {
                return c.edit
            })
        },
        bindModel(bind, dataSource) {
            let object = {}
            if (bind) {
                for (const [key, value] of Object.entries(bind)) {
                    object[key] = this.fn(dataSource, value)
                }
            }
            return object
        },
        getInputPlaceholder(d, dataSource) {
            // edit 模式
            if (dataSource.id) {
                return d.edit.placeholder?.edit ?? d.edit.placeholder
            } else {
                // add 模式
                return d.edit.placeholder?.add ?? d.edit.placeholder
            }
        }
    }
}
</script>

<style scoped>

</style>
