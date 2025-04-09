import type { Column } from '@/components/StdDesign/types'
import { datetime, mask } from '@/components/StdDesign/StdDataDisplay/StdTableTransformer'
import { select } from '@/components/StdDesign/StdDataEntry'
import gettext from '@/gettext'
import configMap from './index'

const languageAvailable = gettext.available

const configTypeMask = Object.keys(configMap).reduce((acc, key) => {
  acc[key] = configMap[key].name()
  return acc
}, {})

const columns: Column[] = [
  {
    dataIndex: 'type',
    title: () => $gettext('Type'),
    customRender: mask(configTypeMask),
    edit: {
      type: select,
      mask: configTypeMask,
      config: {
        required: true,
      },
    },
  },
  {
    dataIndex: 'language',
    title: () => $gettext('Language'),
    customRender: mask(languageAvailable),
    edit: {
      type: select,
      mask: languageAvailable,
      config: {
        required: true,
      },
    },
  },
  {
    dataIndex: 'created_at',
    title: () => $gettext('Created at'),
    customRender: datetime,
  },
  {
    dataIndex: 'action',
    title: () => $gettext('Action'),
  },
]

export default columns
