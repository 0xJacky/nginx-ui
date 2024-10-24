import type { JSX } from 'vue/jsx-runtime'
import { Tag } from 'ant-design-vue'
// text, record, index, column
import dayjs from 'dayjs'
import { get } from 'lodash'

export interface CustomRenderProps {
  // eslint-disable-next-line ts/no-explicit-any
  text: any
  // eslint-disable-next-line ts/no-explicit-any
  record: any
  // eslint-disable-next-line ts/no-explicit-any
  index: any
  // eslint-disable-next-line ts/no-explicit-any
  column: any
  isExport?: boolean
  isDetail?: boolean
}

export function datetime(args: CustomRenderProps) {
  return dayjs(args.text).format('YYYY-MM-DD HH:mm:ss')
}

export function date(args: CustomRenderProps) {
  return args.text ? dayjs(args.text).format('YYYY-MM-DD') : '-'
}

// Used in Export
date.isDate = true
datetime.isDatetime = true

// eslint-disable-next-line ts/no-explicit-any
export function mask(maskObj: any): (args: CustomRenderProps) => JSX.Element {
  return (args: CustomRenderProps) => {
    // eslint-disable-next-line ts/no-explicit-any
    let v: any

    if (typeof maskObj?.[args.text] === 'function')
      v = maskObj[args.text]()
    else if (typeof maskObj?.[args.text] === 'string')
      v = maskObj[args.text]
    else v = args.text

    return v ?? '-'
  }
}

export function arrayToTextRender(args: CustomRenderProps) {
  return args.text?.join(', ')
}
export function actualValueRender(args: CustomRenderProps, actualDataIndex: string | string[]) {
  return get(args.record, actualDataIndex)
}

export function longTextWithEllipsis(len: number): (args: CustomRenderProps) => JSX.Element {
  return (args: CustomRenderProps) => {
    if (args.isExport || args.isDetail)
      return args.text

    return args.text.length > len ? `${args.text.substring(0, len)}...` : args.text
  }
}

export function year(args: CustomRenderProps) {
  return dayjs(args.text).format('YYYY')
}

// eslint-disable-next-line ts/no-explicit-any
export function maskRenderWithColor(maskObj: any) {
  return (args: CustomRenderProps) => {
    let label: string
    if (typeof maskObj[args.text] === 'function')
      label = maskObj[args.text]()
    else if (typeof maskObj[args.text] === 'string')
      label = maskObj[args.text]
    else label = args.text

    if (args.isExport)
      return label

    const colorMap = {
      0: '',
      1: 'blue',
      2: 'green',
      3: 'red',
      4: 'cyan',
    }

    return args.text ? h(Tag, { color: colorMap[args.text] }, maskObj[args.text]) : '-'
  }
}
