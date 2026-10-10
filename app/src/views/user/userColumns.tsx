import type { StdTableColumn } from '@uozi-admin/curd'
import { datetimeRender } from '@uozi-admin/curd'
import { Tag } from 'antdv-next'

const columns: StdTableColumn[] = [{
  title: () => $gettext('Username'),
  dataIndex: 'name',
  sorter: true,
  pure: true,
  edit: {
    type: 'input',
  },
  search: true,
}, {
  title: () => $gettext('Password'),
  dataIndex: 'password',
  sorter: true,
  pure: true,
  edit: {
    type: 'password',
    password: {
      placeholder: $gettext('Leave blank for no change'),
      generate: true,
    },
  },
  hiddenInTable: true,
  hiddenInDetail: true,
}, {
  title: () => $gettext('Require MFA for this user'),
  dataIndex: 'mfa_required',
  edit: { type: 'switch' },
  hiddenInTable: true,
  pure: true,
}, {
  title: () => $gettext('MFA Policy'),
  dataIndex: 'mfa_policy_source',
  customRender: ({ text }) => <Tag color={text === 'optional' ? 'default' : 'blue'}>{text === 'global' ? $gettext('Global enforcement') : text === 'user' ? $gettext('User enforcement') : $gettext('Optional')}</Tag>,
  pure: true,
}, {
  title: () => $gettext('MFA Enrollment'),
  dataIndex: 'mfa_pending',
  customRender: ({ record }) => <Tag color={record.mfa_pending ? 'orange' : record.enabled_2fa ? 'green' : 'default'}>{record.mfa_pending ? $gettext('Pending enrollment') : record.enabled_2fa ? $gettext('Enabled') : $gettext('Disabled')}</Tag>,
  pure: true,
}, {
  title: () => $gettext('Created at'),
  dataIndex: 'created_at',
  customRender: datetimeRender,
  sorter: true,
  pure: true,
}, {
  title: () => $gettext('Updated at'),
  dataIndex: 'updated_at',
  customRender: datetimeRender,
  sorter: true,
  pure: true,
}, {
  title: () => $gettext('Actions'),
  dataIndex: 'actions',
  fixed: 'right',
  width: 340,
}]

export default columns
