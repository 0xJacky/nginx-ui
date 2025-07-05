import type { CustomRenderArgs, StdTableColumn } from '@uozi-admin/curd'
import { datetimeRender } from '@uozi-admin/curd'

const configColumns: StdTableColumn[] = [{
  title: () => $gettext('Search'),
  dataIndex: 'search',
  search: {
    type: 'input',
    input: {
      placeholder: $gettext('Name or content'),
    },
  },
  hiddenInEdit: true,
  hiddenInTable: true,
  hiddenInDetail: true,
}, {
  title: () => $gettext('Name'),
  dataIndex: 'name',
  sorter: true,
  pure: true,
  customRender: ({ text, record }: CustomRenderArgs) => {
    function renderIcon(isDir: boolean) {
      return (
        <div class="mr-2 text-truegray-5">
          {isDir
            ? <div class="i-tabler-folder-filled" />
            : <div class="i-tabler-file" />}
        </div>
      )
    }

    const displayName = text || ''

    return (
      <div class="flex">
        {renderIcon(record.is_dir)}
        {displayName}
      </div>
    )
  },
  width: 500,
}, {
  title: () => $gettext('Updated at'),
  dataIndex: 'modified_at',
  customRender: datetimeRender,
  sorter: true,
  pure: true,
  width: 200,
}, {
  title: () => $gettext('Actions'),
  dataIndex: 'actions',
  fixed: 'right',
  width: 180,
}]

export default configColumns
