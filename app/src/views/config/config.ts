import { h } from 'vue'
import type { customRender } from '@/components/StdDesign/StdDataDisplay/StdTableTransformer'
import { datetime } from '@/components/StdDesign/StdDataDisplay/StdTableTransformer'
import gettext from '@/gettext'

const { $gettext } = gettext

const configColumns = [{
  title: () => $gettext('Name'),
  dataIndex: 'name',
  sorter: true,
  pithy: true,
}, {
  title: () => $gettext('Type'),
  dataIndex: 'is_dir',
  customRender: (args: customRender) => {
    const template = []
    const { text } = args
    if (text === true || text > 0)
      template.push($gettext('Directory'))
    else
      template.push($gettext('File'))

    return h('div', template)
  },
  sorter: true,
  pithy: true,
}, {
  title: () => $gettext('Updated at'),
  dataIndex: 'modified_at',
  customRender: datetime,
  datetime: true,
  sorter: true,
  pithy: true,
}, {
  title: () => $gettext('Action'),
  dataIndex: 'action',
}]

export default configColumns
