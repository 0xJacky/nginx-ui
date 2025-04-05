import type { Column } from '@/components/StdDesign/types'
import { PostSyncAction } from '@/api/env_group'
import { datetime } from '@/components/StdDesign/StdDataDisplay/StdTableTransformer'
import { input } from '@/components/StdDesign/StdDataEntry'

const columns: Column[] = [{
  dataIndex: 'name',
  title: () => $gettext('Name'),
  search: true,
  edit: {
    type: input,
  },
  handle: true,
  pithy: true,
  width: 120,
}, {
  title: () => $gettext('Post-sync Action'),
  dataIndex: 'post_sync_action',
  customRender: ({ text }) => {
    if (!text || text === PostSyncAction.None) {
      return $gettext('No Action')
    }
    else if (text === PostSyncAction.ReloadNginx) {
      return $gettext('Reload Nginx')
    }
    return text
  },
  pithy: true,
  width: 150,
}, {
  title: () => $gettext('Created at'),
  dataIndex: 'created_at',
  customRender: datetime,
  pithy: true,
  width: 150,
}, {
  title: () => $gettext('Updated at'),
  dataIndex: 'updated_at',
  customRender: datetime,
  pithy: true,
  width: 150,
}, {
  title: () => $gettext('Action'),
  dataIndex: 'action',
  fixed: 'right',
  width: 150,
}]

export default columns
