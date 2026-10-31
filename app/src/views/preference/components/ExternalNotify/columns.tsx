import type { CustomRenderArgs, StdTableColumn } from '@uozi-admin/curd'
import type { ExternalNotify } from '@/api/external_notify'
import { datetimeRender, maskRender } from '@uozi-admin/curd'
import gettext from '@/gettext'
import EnabledSwitch from './EnabledSwitch.vue'
import ExternalNotifyEditor from './ExternalNotifyEditor.vue'
import configMap from './index'
import { pluginChannels } from './pluginChannels'

const languageAvailable = gettext.available

// Reactive, so the plugin channels appear once they are loaded.
const configTypeMask = reactive<Record<string, string>>(Object.keys(configMap).reduce((acc, key) => {
  acc[key] = configMap[key].name()
  return acc
}, {} as Record<string, string>))

watch(pluginChannels, channels => {
  for (const channel of channels)
    configTypeMask[channel.type] = channel.name
}, { immediate: true })

const columns: StdTableColumn[] = [
  {
    dataIndex: 'index',
    title: () => $gettext('Index'),
    customRender: ({ record }: CustomRenderArgs<ExternalNotify>) => record.id,
    width: 80,
  },
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
    dataIndex: 'description',
    title: () => $gettext('Description'),
    edit: {
      type: 'input',
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
      type: (context: { formData: ExternalNotify }) => {
        if (!context.formData.type) {
          return <div />
        }

        if (!context.formData.config) {
          context.formData.config = {}
        }
        return (
          <ExternalNotifyEditor v-model={context.formData.config} type={context.formData.type} />
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
