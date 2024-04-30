// text, record, index, column
import dayjs from 'dayjs'
import type { JSX } from 'vue/jsx-runtime'
import { Tag } from 'ant-design-vue'
import { get } from 'lodash'

export interface customRender {
  // eslint-disable-next-line @typescript-eslint/no-explicit-any
  text: any
  // eslint-disable-next-line @typescript-eslint/no-explicit-any
  record: any
  // eslint-disable-next-line @typescript-eslint/no-explicit-any
  index: any
  // eslint-disable-next-line @typescript-eslint/no-explicit-any
  column: any
  isExport?: boolean
  isDetail?: boolean
}

export const datetime = (args: customRender) => {
  return dayjs(args.text).format('YYYY-MM-DD HH:mm:ss')
}

export const date = (args: customRender) => {
  return args.text ? dayjs(args.text).format('YYYY-MM-DD') : '-'
}

// Used in Export
date.isDate = true
datetime.isDatetime = true

// eslint-disable-next-line @typescript-eslint/no-explicit-any
export const mask = (maskObj: any): (args: customRender) => JSX.Element => {
  return (args: customRender) => {
    // eslint-disable-next-line @typescript-eslint/no-explicit-any
    let v: any

    if (typeof maskObj?.[args.text] === 'function')
      v = maskObj[args.text]()
    else if (typeof maskObj?.[args.text] === 'string')
      v = maskObj[args.text]
    else v = args.text

    return v ?? '-'
  }
}

export const arrayToTextRender = (args: customRender) => {
  return args.text?.join(', ')
}
export const actualValueRender = (args: customRender, actualDataIndex: string | string[]) => {
  return get(args.record, actualDataIndex)
}

export const longTextWithEllipsis = (len: number): (args: customRender) => JSX.Element => {
  return (args: customRender) => {
    if (args.isExport || args.isDetail)
      return args.text

    return args.text.length > len ? `${args.text.substring(0, len)}...` : args.text
  }
}

export const year = (args: customRender) => {
  return dayjs(args.text).format('YYYY')
}

// eslint-disable-next-line @typescript-eslint/no-explicit-any
export const maskRenderWithColor = (maskObj: any) => (args: customRender) => {
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
