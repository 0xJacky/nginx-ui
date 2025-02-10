import type {
  CustomRender,
} from '@/components/StdDesign/StdDataDisplay/StdTableTransformer'
import type { Column, JSXElements } from '@/components/StdDesign/types'
import site_category from '@/api/site_category'
import {
  actualValueRender,
  datetime,
} from '@/components/StdDesign/StdDataDisplay/StdTableTransformer'
import { input, select, selector } from '@/components/StdDesign/StdDataEntry'
import siteCategoryColumns from '@/views/site/site_category/columns'
import { Badge } from 'ant-design-vue'

const columns: Column[] = [{
  title: () => $gettext('Name'),
  dataIndex: 'name',
  sorter: true,
  pithy: true,
  edit: {
    type: input,
  },
  search: true,
  width: 120,
}, {
  title: () => $gettext('Category'),
  dataIndex: 'site_category_id',
  customRender: actualValueRender('site_category.name'),
  edit: {
    type: selector,
    selector: {
      api: site_category,
      columns: siteCategoryColumns,
      recordValueIndex: 'name',
      selectionType: 'radio',
    },
  },
  sorter: true,
  pithy: true,
  batch: true,
  width: 100,
}, {
  title: () => $gettext('Status'),
  dataIndex: 'enabled',
  customRender: (args: CustomRender) => {
    const template: JSXElements = []
    const { text } = args
    if (text === true || text > 0) {
      template.push(<Badge status="success" />)
      template.push($gettext('Enabled'))
    }
    else {
      template.push(<Badge status="warning" />)
      template.push($gettext('Disabled'))
    }

    return h('div', template)
  },
  search: {
    type: select,
    mask: {
      true: $gettext('Enabled'),
      false: $gettext('Disabled'),
    },
  },
  sorter: true,
  pithy: true,
  width: 80,
}, {
  title: () => $gettext('Updated at'),
  dataIndex: 'modified_at',
  customRender: datetime,
  sorter: true,
  pithy: true,
  width: 150,
}, {
  title: () => $gettext('Action'),
  dataIndex: 'action',
  width: 80,
  fixed: 'right',
}]

export default columns
