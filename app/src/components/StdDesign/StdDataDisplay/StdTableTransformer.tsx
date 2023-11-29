// text, record, index, column
import dayjs from 'dayjs'

export interface customRender {
  // eslint-disable-next-line @typescript-eslint/no-explicit-any
  text: any
  // eslint-disable-next-line @typescript-eslint/no-explicit-any
  record: any
  // eslint-disable-next-line @typescript-eslint/no-explicit-any
  index: any
  // eslint-disable-next-line @typescript-eslint/no-explicit-any
  column: any
}

export const datetime = (args: customRender) => {
  return dayjs(args.text).format('YYYY-MM-DD HH:mm:ss')
}

export const date = (args: customRender) => {
  return dayjs(args.text).format('YYYY-MM-DD')
}
// eslint-disable-next-line @typescript-eslint/no-explicit-any
export const mask = (args: customRender, maskObj: any) => {
  let v

  if (typeof maskObj?.[args.text] === 'function')
    v = maskObj[args.text]()
  else if (typeof maskObj?.[args.text] === 'string')
    v = maskObj[args.text]
  else
    v = args.text

  return <div>{v}</div>
}
