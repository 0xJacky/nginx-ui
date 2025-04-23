import type { CustomRender } from '@/components/StdDesign/StdDataDisplay/StdTableTransformer'
import { get } from 'lodash'

// eslint-disable-next-line ts/no-redeclare
export function CustomRender(props: CustomRender) {
  return props.column.customRender
    ? props.column.customRender(props)
    : get(props.record, props.column.dataIndex!)
}
