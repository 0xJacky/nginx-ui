// text, record, index, column
import dayjs from 'dayjs'

export interface customRender {
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
