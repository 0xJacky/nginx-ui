import type { StdTableProps } from '@/components/StdDesign/StdDataDisplay/types'
import type { Key } from 'ant-design-vue/es/_util/type'
import type { Ref } from 'vue'
import { message } from 'ant-design-vue'
import sortable from 'sortablejs'

// eslint-disable-next-line ts/no-explicit-any
function getRowKey(item: any) {
  return item.dataset.rowKey
}

// eslint-disable-next-line ts/no-explicit-any
function getTargetData(data: any, indexList: number[]): any {
  // eslint-disable-next-line ts/no-explicit-any
  let target: any = { children: data }
  indexList.forEach((index: number) => {
    target.children[index].parent = target
    target = target.children[index]
  })

  return target
}

// eslint-disable-next-line ts/no-explicit-any
function useSortable(props: StdTableProps, randomId: Ref<string>, dataSource: Ref<any[]>, rowsKeyIndexMap: Ref<Record<number, number[]>>, expandKeysList: Ref<Key[]>) {
  // eslint-disable-next-line ts/no-explicit-any
  const table: any = document.querySelector(`#std-table-${randomId.value} tbody`)

  // eslint-disable-next-line no-new,new-cap,sonarjs/constructor-for-side-effects
  new sortable(table, {
    handle: '.ant-table-drag-icon',
    animation: 150,
    sort: true,
    forceFallback: true,
    setData(dataTransfer) {
      dataTransfer.setData('Text', '')
    },
    onStart({ item }) {
      const targetRowKey = Number(getRowKey(item))
      if (targetRowKey)
        expandKeysList.value = expandKeysList.value.filter((_item: Key) => _item !== targetRowKey)
    },
    onMove({
      dragged,
      related,
    }) {
      const oldRow: number[] = rowsKeyIndexMap.value?.[Number(getRowKey(dragged))]
      const newRow: number[] = rowsKeyIndexMap.value?.[Number(getRowKey(related))]

      if (oldRow.length !== newRow.length || oldRow[oldRow.length - 2] !== newRow[newRow.length - 2])
        return false

      if (props.sortableMoveHook)
        return props.sortableMoveHook(oldRow, newRow)
    },
    async onEnd({
      item,
      newIndex,
      oldIndex,
    }) {
      if (newIndex === oldIndex)
        return

      const indexDelta: number = Number(oldIndex) - Number(newIndex)
      const direction: number = indexDelta > 0 ? +1 : -1

      const rowIndex: number[] = rowsKeyIndexMap.value?.[Number(getRowKey(item))]
      const newRow = getTargetData(dataSource.value, rowIndex)
      const newRowParent = newRow.parent
      const level: number = newRow.level

      const currentRowIndex: number[] = [...rowsKeyIndexMap.value![Number(getRowKey(table?.children?.[Number(newIndex) + direction]))]]

      // eslint-disable-next-line ts/no-explicit-any
      const currentRow: any = getTargetData(dataSource.value, currentRowIndex)

      // Reset parent
      currentRow.parent = newRow.parent = null
      newRowParent.children.splice(rowIndex[level], 1)
      newRowParent.children.splice(currentRowIndex[level], 0, toRaw(newRow))

      const changeIds: number[] = []

      // eslint-disable-next-line ts/no-explicit-any
      function processChanges(row: any, children = false, _newIndex: number | undefined = undefined) {
        // Build changes ID list expect new row
        if (children || _newIndex === undefined)
          changeIds.push(row.id)

        if (_newIndex !== undefined)
          rowsKeyIndexMap.value[row.id][level] = _newIndex
        else if (children)
          rowsKeyIndexMap.value[row.id][level] += direction

        row.parent = null
        if (row.children)
        // eslint-disable-next-line ts/no-explicit-any
          row.children.forEach((v: any) => processChanges(v, true, _newIndex))
      }

      // Replace row index for new row
      processChanges(newRow, false, currentRowIndex[level])

      // Rebuild row index maps for changes row
      // eslint-disable-next-line sonarjs/no-equals-in-for-termination
      for (let i = Number(oldIndex); i !== newIndex; i -= direction) {
        const _rowIndex: number[] = rowsKeyIndexMap.value?.[getRowKey(table.children[i])]

        _rowIndex[level] += direction
        processChanges(getTargetData(dataSource.value, _rowIndex))
      }

      // console.log('Change row id', newRow.id, 'order', newRow.id, '=>', currentRow.id, ', direction: ', direction,
      //   ', changes IDs:', changeIds

      props.api.update_order({
        target_id: newRow.id,
        direction,
        affected_ids: changeIds,
      }).then(() => {
        message.success($gettext('Updated successfully'))
        // eslint-disable-next-line ts/no-explicit-any
      }).catch((e: any) => {
        message.error(e?.message ?? $gettext('Server error'))
      })
    },
  })
}

export default useSortable
