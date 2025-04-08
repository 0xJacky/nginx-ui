import type { VNode } from 'vue'
import type { JSX } from 'vue/jsx-runtime'
import { CopyOutlined } from '@ant-design/icons-vue'
import { message, Tag } from 'ant-design-vue'
// text, record, index, column
import dayjs from 'dayjs'
import { get } from 'lodash'

// eslint-disable-next-line ts/no-explicit-any
export interface CustomRender<T = any, R = any> {
  text: T
  record: R
  // eslint-disable-next-line ts/no-explicit-any
  index: any
  // eslint-disable-next-line ts/no-explicit-any
  column: any
  isExport?: boolean
  isDetail?: boolean
}

export function datetime(args: CustomRender) {
  if (!args.text)
    return '/'

  return dayjs(args.text).format('YYYY-MM-DD HH:mm:ss')
}

export function date(args: CustomRender) {
  return args.text ? dayjs(args.text).format('YYYY-MM-DD') : '-'
}

// Used in Export
date.isDate = true
datetime.isDatetime = true

// eslint-disable-next-line ts/no-explicit-any
export function mask(maskObj: any): (args: CustomRender) => JSX.Element {
  return (args: CustomRender) => {
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

export function arrayToTextRender(args: CustomRender) {
  return args.text?.join(', ')
}
export function actualValueRender(actualDataIndex: string | string[]) {
  return (args: CustomRender) => {
    return get(args.record, actualDataIndex) || ''
  }
}

export function longTextWithEllipsis(len: number): (args: CustomRender) => JSX.Element {
  return (args: CustomRender) => {
    if (args.isExport || args.isDetail)
      return args.text

    return args.text.length > len ? `${args.text.substring(0, len)}...` : args.text
  }
}

// eslint-disable-next-line ts/no-explicit-any
export function maskRenderWithColor(maskObj: any, customColors?: Record<string | number, string> | string) {
  return (args: CustomRender) => {
    let label: string
    if (typeof maskObj[args.text] === 'function')
      label = maskObj[args.text]()
    else if (typeof maskObj[args.text] === 'string')
      label = maskObj[args.text]
    else label = args.text

    if (args.isExport)
      return label

    let colorMap: Record<string | number, string> = {
      0: '',
      1: 'blue',
      2: 'green',
      3: 'purple',
      4: 'cyan',
    }

    if (typeof customColors === 'object')
      colorMap = customColors

    let color = colorMap[args.text]

    if (typeof customColors === 'string')
      color = customColors

    return args.text ? h(Tag, { color }, () => label) : '/'
  }
}

interface MultiFieldRenderProps {
  key: string | number | string[] | number[]
  label?: () => string
  prefix?: string
  suffix?: string
  render?: ((args: CustomRender) => string | number | VNode) | (() => ((args: CustomRender) => string | VNode))
  direction?: 'vertical' | 'horizontal'
}

export function multiFieldsRender(fields: MultiFieldRenderProps[]) {
  return (args: CustomRender) => {
    const list = fields.map(field => {
      let label = field.label?.()
      let value = get(args.record, field.key)

      if (field.prefix)
        value = field.prefix + value
      if (field.suffix)
        value += field.suffix

      if (label)
        label += ':'

      const valueNode = field.render?.({ ...args, text: value }) ?? value
      const direction = field.direction ?? 'vertical'

      const labelNode = label
        // eslint-disable-next-line sonarjs/no-nested-conditional
        ? h(direction === 'vertical' ? 'div' : 'span', { class: 'text-gray-500 my-1 mr-1' }, label)
        : null

      return h('div', { class: 'my-4' }, [labelNode, valueNode])
    })

    return h('div', null, list)
  }
}

export function copiableFieldRender(args: CustomRender) {
  return h('div', null, [
    h('span', null, args.text),
    h(CopyOutlined, {
      style: {
        marginLeft: '10px',
        cursor: 'pointer',
      },
      onClick: () => {
        navigator.clipboard.writeText(args.text).then(() => {
          message.success($gettext('Copied'))
        })
      },
    }),
  ])
}
