// text, record, index, column
import dayjs from 'dayjs'

export interface customRender {
  value: any
  text: any
  record: any
  index: any
  column: any
}

export const datetime = (args: customRender) => {
  return dayjs(args.text).format('YYYY-MM-DD HH:mm:ss')
}

export const date = (args: customRender) => {
  return dayjs(args.text).format('YYYY-MM-DD')
}

export const mask = (args: customRender, maskObj: any) => {
  let v

  if (typeof maskObj?.[args.text] === 'function') {
    v = maskObj[args.text]()
  } else if (typeof maskObj?.[args.text] === 'string') {
    v = maskObj[args.text]
  } else {
    v = args.text
  }

  return <div>{v}</div>
}
