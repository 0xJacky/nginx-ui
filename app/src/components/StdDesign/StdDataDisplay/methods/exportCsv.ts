import { message } from 'ant-design-vue'
import dayjs from 'dayjs'
import type { ComputedRef } from 'vue'
import _ from 'lodash'
import { downloadCsv } from '@/lib/helper'
import type { Column, StdTableResponse } from '@/components/StdDesign/types'
import gettext from '@/gettext'
import type { StdTableProps } from '@/components/StdDesign/StdDataDisplay/StdTable.vue'

const { $gettext } = gettext
async function exportCsv(props: StdTableProps, pithyColumns: ComputedRef<Column[]>) {
  const header: { title?: string; key: string | string[] }[] = []
  // eslint-disable-next-line @typescript-eslint/no-explicit-any
  const headerKeys: any[] = []
  const showColumnsMap: Record<string, Column> = {}

  pithyColumns.value.forEach((column: Column) => {
    if (column.dataIndex === 'action')
      return
    let t = column.title
    if (typeof t === 'function')
      t = t()
    header.push({
      title: t,
      key: column.dataIndex,
    })
    headerKeys.push(column.dataIndex.toString())
    showColumnsMap[column.dataIndex.toString()] = column
  })

  // eslint-disable-next-line @typescript-eslint/no-explicit-any
  const dataSource: any[] = []
  let hasMore = true
  let page = 1
  while (hasMore) {
    // 准备 DataSource
    await props.api!.get_list({ page }).then((r: StdTableResponse) => {
      if (r.data.length === 0) {
        hasMore = false

        return
      }
      dataSource.push(...r.data)
    }).catch((e: { message?: string }) => {
      message.error(e.message ?? $gettext('Server error'))
      hasMore = false
    })
    page += 1
  }
  // eslint-disable-next-line @typescript-eslint/no-explicit-any
  const data: any[] = []

  dataSource.forEach(row => {
    // eslint-disable-next-line @typescript-eslint/no-explicit-any
    const obj: Record<string, any> = {}

    headerKeys.forEach(key => {
      let _data = _.get(row, key)
      const c = showColumnsMap[key]

      _data = c?.customRender?.({ text: _data }) ?? _data
      _.set(obj, c.dataIndex, _data)
    })
    data.push(obj)
  })

  downloadCsv(header, data,
    `${$gettext('Export')}-${props.title}-${dayjs().format('YYYYMMDDHHmmss')}.csv`)
}

export default exportCsv
