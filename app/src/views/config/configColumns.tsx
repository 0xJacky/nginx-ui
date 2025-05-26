import type { CustomRenderArgs, StdTableColumn } from '@uozi-admin/curd'
import { FileFilled, FolderFilled } from '@ant-design/icons-vue'
import { datetimeRender } from '@uozi-admin/curd'

const configColumns: StdTableColumn[] = [{
  title: () => $gettext('Name'),
  dataIndex: 'name',
  sorter: true,
  pure: true,
  search: {
    type: 'input',
  },
  customRender: ({ text, record }: CustomRenderArgs) => {
    function renderIcon(isDir: boolean) {
      return (
        <div class="mr-2 text-truegray-5">
          {isDir
            ? <FolderFilled />
            : <FileFilled />}
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
