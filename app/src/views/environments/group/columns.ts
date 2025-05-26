import type { StdTableColumn } from '@uozi-admin/curd'
import { datetimeRender } from '@uozi-admin/curd'
import { PostSyncAction } from '@/api/env_group'

const columns: StdTableColumn[] = [{
  dataIndex: 'name',
  title: () => $gettext('Name'),
  search: true,
  edit: {
    type: 'input',
  },
  pure: true,
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
  pure: true,
  width: 150,
}, {
  title: () => $gettext('Created at'),
  dataIndex: 'created_at',
  customRender: datetimeRender,
  pure: true,
  width: 150,
}, {
  title: () => $gettext('Updated at'),
  dataIndex: 'updated_at',
  customRender: datetimeRender,
  pure: true,
  width: 150,
}, {
  title: () => $gettext('Actions'),
  dataIndex: 'actions',
  fixed: 'right',
  width: 150,
}]

export default columns
