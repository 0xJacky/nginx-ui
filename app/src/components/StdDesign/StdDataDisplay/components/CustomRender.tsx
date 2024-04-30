import _ from 'lodash'
import type { customRender } from '@/components/StdDesign/StdDataDisplay/StdTableTransformer'

export function CustomRender(props: customRender) {
  return props.column.customRender
    ? props.column.customRender(props)
    : _.get(props.record, props.column.dataIndex!)
}
