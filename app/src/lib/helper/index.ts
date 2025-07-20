import dayjs from 'dayjs'
import relativeTime from 'dayjs/plugin/relativeTime'

function bytesToSize(bytes: number) {
  if (bytes === 0)
    return '0 B'

  const k = 1024

  const sizes = ['B', 'KiB', 'MiB', 'GiB', 'TiB', 'PiB', 'EiB', 'ZiB', 'YiB']

  const i = Math.floor(Math.log(bytes) / Math.log(k))

  return `${(bytes / k ** i).toFixed(2)} ${sizes[i]}`
}
// eslint-disable-next-line ts/no-explicit-any
function downloadCsv(header: any, data: any[], fileName: string) {
  if (!header || !Array.isArray(header) || !Array.isArray(data) || !header.length)
    return

  let csvContent = 'data:text/csv;charset=utf-8,\uFEFF'
  const _header = header.map(h => h.title).join(',')
  const keys = header.map(item => item.key)

  csvContent += `${_header}\n`
  data.forEach((item, index) => {
    let dataString = ''
    for (const element of keys)
      dataString += `${item[element]},`

    csvContent += index < data.length ? dataString.replace(/,$/, '\n') : dataString.replace(/,$/, '')
  })

  const a = document.createElement('a')

  a.href = encodeURI(csvContent)
  a.download = fileName
  a.click()
  window.URL.revokeObjectURL(csvContent)
}

function urlJoin(...args: string[]) {
  return args
    .filter(arg => arg)
    .join('/')
    .replace(/\/+/g, '/')
    .replace(/^(.+):\//, '$1://')
    .replace(/^file:/, 'file:/')
    .replace(/\/(\?|&|#[^!])/g, '$1')
    .replace(/\?/g, '&')
    .replace('&', '?')
}

function fromNow(t: string) {
  dayjs.extend(relativeTime)

  return dayjs(t).fromNow()
}

function formatDate(t: string) {
  return dayjs(t).format('YYYY.MM.DD')
}

function formatDateTime(t: string) {
  return dayjs(t).format('YYYY-MM-DD HH:mm:ss')
}

export {
  bytesToSize,
  downloadCsv,
  formatDate,
  formatDateTime,
  fromNow,
  urlJoin,
}

export { clearFingerprintCache, getBrowserFingerprint } from './fingerprint'
