import type { CustomRenderProps } from '@/components/StdDesign/StdDataDisplay/StdTableTransformer'
import _ from 'lodash'

export function CustomRender(props: CustomRenderProps) {
  return props.column.customRender
    ? props.column.customRender(props)
    : _.get(props.record, props.column.dataIndex!)
}
