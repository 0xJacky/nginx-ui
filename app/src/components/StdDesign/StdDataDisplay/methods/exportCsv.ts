import type { GetListResponse } from '@/api/curd'
import type { StdTableProps } from '@/components/StdDesign/StdDataDisplay/StdTable.vue'
import type { Column } from '@/components/StdDesign/types'
import type { ComputedRef } from 'vue'
import { downloadCsv } from '@/lib/helper'
import { message } from 'ant-design-vue'
import dayjs from 'dayjs'
import _ from 'lodash'

async function exportCsv(props: StdTableProps, pithyColumns: ComputedRef<Column[]>) {
  const header: { title?: string, key: Column['dataIndex'] }[] = []
  // eslint-disable-next-line ts/no-explicit-any
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
    headerKeys.push(column?.dataIndex?.toString())
    showColumnsMap[column?.dataIndex?.toString() as string] = column
  })

  // eslint-disable-next-line ts/no-explicit-any
  const dataSource: any[] = []
  let hasMore = true
  let page = 1
  while (hasMore) {
    // 准备 DataSource
    await props
    // eslint-disable-next-line ts/no-explicit-any
      .api!.get_list({ page }).then((r: GetListResponse<any>) => {
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
  // eslint-disable-next-line ts/no-explicit-any
  const data: any[] = []

  dataSource.forEach(row => {
    // eslint-disable-next-line ts/no-explicit-any
    const obj: Record<string, any> = {}

    headerKeys.forEach(key => {
      let _data = _.get(row, key)
      const c = showColumnsMap[key]

      _data = c?.customRender?.({ text: _data }) ?? _data
      _.set(obj, c.dataIndex as string, _data)
    })
    data.push(obj)
  })

  downloadCsv(header, data, `${$gettext('Export')}-${props.title}-${dayjs().format('YYYYMMDDHHmmss')}.csv`)
}

export default exportCsv
