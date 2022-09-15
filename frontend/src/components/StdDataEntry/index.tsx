import StdDataEntry from './StdDataEntry.js'
import {h} from 'vue'
import {Input, Textarea, InputPassword} from 'ant-design-vue'
import StdSelector from './compontents/StdSelector.vue'
import StdSelect from './compontents/StdSelect.vue'
import StdPassword from './compontents/StdPassword.vue'

interface IEdit {
    type: Function
    placeholder: any
    mask: any
    key: any
    value: any
    recordValueIndex: any
    selectionType: any
    api: Object,
    columns: any,
    data_key: any,
    disable_search: boolean,
    get_params: Object,
    description: string
    generate: boolean
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
        }
    })
}

function textarea(edit: IEdit, dataSource: any, dataIndex: any) {
    return h(Textarea, {
        placeholder: edit.placeholder?.() ?? '',
        value: dataSource?.[dataIndex],
        'onUpdate:value': value => {
            dataSource[dataIndex] = value
        }
    })
}

function password(edit: IEdit, dataSource: any, dataIndex: any) {
    return <StdPassword
        v-model:value={dataSource[dataIndex]}
        generate={edit.generate}
        placeholder={edit.placeholder}
    />
}

function select(edit: IEdit, dataSource: any, dataIndex: any) {
    return <StdSelect
        v-model:value={dataSource[dataIndex]}
        mask={edit.mask}
    />
}

function selector(edit: IEdit, dataSource: any, dataIndex: any) {
    return <StdSelector
        v-model:selectedKey={dataSource[dataIndex]}
        value={edit.value}
        recordValueIndex={edit.recordValueIndex}
        selectionType={edit.selectionType}
        api={edit.api}
        columns={edit.columns}
        data_key={edit.data_key}
        disable_search={edit.disable_search}
        get_params={edit.get_params}
        description={edit.description}
    />
}

export {
    readonly,
    input,
    textarea,
    select,
    selector,
    password
}

export default StdDataEntry
