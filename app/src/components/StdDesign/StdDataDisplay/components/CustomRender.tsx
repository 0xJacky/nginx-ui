import type { CustomRender } from '@/components/StdDesign/StdDataDisplay/StdTableTransformer'
import _ from 'lodash'

// eslint-disable-next-line ts/no-redeclare,sonarjs/no-redeclare
export function CustomRender(props: CustomRender) {
  return props.column.customRender
    ? props.column.customRender(props)
    : _.get(props.record, props.column.dataIndex!)
}
