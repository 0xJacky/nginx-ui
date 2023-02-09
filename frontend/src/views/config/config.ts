import { customRender, datetime } from '@/components/StdDataDisplay/StdTableTransformer'
import gettext from '@/gettext'

const { $gettext } = gettext

import { h } from 'vue'

import { useRouter } from 'vue-router'

const router = useRouter()

const configColumns = [{
    title: () => $gettext('Name'),
    dataIndex: 'name',
    sorter: true,
    pithy: true
}, {
    title: () => $gettext('Type'),
    dataIndex: 'is_dir',
    customRender: (args: customRender) => {
        const template: any = []
        const { text, column } = args
        console.log("args....", args)
        if (text === true || text > 0) {
            template.push($gettext('Dir'))
        } else {
            template.push($gettext('File'))
        }
        return h('div', template)
    },
    sorter: true,
    pithy: true
}, {
    title: () => $gettext('Updated at'),
    dataIndex: 'modify',
    customRender: datetime,
    datetime: true,
    sorter: true,
    pithy: true
},
{
    title: () => $gettext('action'),
    dataIndex: 'action',
}
]

export default configColumns
