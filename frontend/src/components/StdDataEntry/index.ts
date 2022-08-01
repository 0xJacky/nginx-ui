import StdDataEntry from './StdDataEntry.js'
import {h} from 'vue'
import {Input} from 'ant-design-vue'

interface IEdit {
    type: Function
    placeholder: any
}

function readonly(edit: IEdit, dataSource: any, dataIndex: any) {
    return h('p', dataSource[dataIndex])
}

function input(edit: IEdit, dataSource: any, dataIndex: any) {
    return h(Input, {
        placeholder: edit.placeholder?.() ?? '',
        value: dataSource?.[dataIndex],
        'onUpdate:value': value => {
            dataSource[dataIndex] = value
            // Object.assign(dataSource, {[dataIndex]: value})
        }
    })
}

function textarea(edit: IEdit, dataSource: any, dataIndex: any) {
    return h('a-textarea')
}

function select(edit: IEdit, dataSource: any, dataIndex: any) {
    return h('a-select')
}

export {
    readonly,
    input,
    textarea,
    select
}

export default StdDataEntry
