import type { StdTableColumn } from '@uozi-admin/curd'
import type { ExternalNotify } from '@/api/external_notify'
import { datetimeRender, maskRender } from '@uozi-admin/curd'
import gettext from '@/gettext'
import EnabledSwitch from './EnabledSwitch.vue'
import ExternalNotifyEditor from './ExternalNotifyEditor.vue'
import configMap from './index'

const languageAvailable = gettext.available

const configTypeMask = Object.keys(configMap).reduce((acc, key) => {
  acc[key] = configMap[key].name()
  return acc
}, {})

const columns: StdTableColumn[] = [
  {
    dataIndex: 'type',
    title: () => $gettext('Type'),
    customRender: maskRender(configTypeMask),
    edit: {
      type: 'select',
      select: {
        mask: configTypeMask,
      },
      formItem: {
        required: true,
      },
    },
  },
  {
    dataIndex: 'language',
    title: () => $gettext('Language'),
    customRender: maskRender(languageAvailable),
    edit: {
      type: 'select',
      select: {
        mask: languageAvailable,
      },
      formItem: {
        required: true,
      },
    },
  },
  {
    dataIndex: 'enabled',
    title: () => $gettext('Enabled'),
    customRender: ({ record }: { record: ExternalNotify }) => (
      <EnabledSwitch v-model:enabled={record.enabled} record={record} />
    ),
    edit: {
      type: 'switch',
    },
    width: 100,
  },
  {
    dataIndex: 'config',
    title: () => $gettext('Config'),
    edit: {
      type: (formData: ExternalNotify) => {
        if (!formData.type) {
          return <div />
        }

        if (!formData.config) {
          formData.config = {}
        }
        return (
          <ExternalNotifyEditor v-model={formData.config} type={formData.type} />
        )
      },
      formItem: {
        hiddenLabelInEdit: true,
      },
    },
    hiddenInTable: true,
  },
  {
    dataIndex: 'created_at',
    title: () => $gettext('Created at'),
    customRender: datetimeRender,
  },
  {
    dataIndex: 'actions',
    title: () => $gettext('Actions'),
  },
]

export default columns
