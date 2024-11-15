import type { CustomRender } from '@/components/StdDesign/StdDataDisplay/StdTableTransformer'
import type { JSXElements } from '@/components/StdDesign/types'
import { datetime } from '@/components/StdDesign/StdDataDisplay/StdTableTransformer'
import { input } from '@/components/StdDesign/StdDataEntry'
import { h } from 'vue'

const configColumns = [{
  title: () => $gettext('Name'),
  dataIndex: 'name',
  sorter: true,
  pithy: true,
  search: {
    type: input,
  },
}, {
  title: () => $gettext('Type'),
  dataIndex: 'is_dir',
  customRender: (args: CustomRender) => {
    const template: JSXElements = []
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
