import type { Column } from '@/components/StdDesign/types'
import { datetime } from '@/components/StdDesign/StdDataDisplay/StdTableTransformer'
import { input } from '@/components/StdDesign/StdDataEntry'

const columns: Column[] = [{
  dataIndex: 'name',
  title: () => $gettext('Name'),
  search: true,
  edit: {
    type: input,
  },
  pithy: true,
  width: 120,
}, {
  title: () => $gettext('Created at'),
  dataIndex: 'created_at',
  customRender: datetime,
  sorter: true,
  pithy: true,
  width: 150,
}, {
  title: () => $gettext('Updated at'),
  dataIndex: 'updated_at',
  customRender: datetime,
  sorter: true,
  pithy: true,
  width: 150,
}, {
  title: () => $gettext('Action'),
  dataIndex: 'action',
  fixed: 'right',
  width: 150,
}]

export default columns
