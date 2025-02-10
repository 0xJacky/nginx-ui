import type { CustomRender } from '@/components/StdDesign/StdDataDisplay/StdTableTransformer'
import { datetime } from '@/components/StdDesign/StdDataDisplay/StdTableTransformer'
import { input } from '@/components/StdDesign/StdDataEntry'
import { FileFilled, FolderFilled } from '@ant-design/icons-vue'

const configColumns = [{
  title: () => $gettext('Name'),
  dataIndex: 'name',
  sorter: true,
  pithy: true,
  search: {
    type: input,
  },
  customRender: (args: CustomRender) => {
    function renderIcon(isDir: boolean) {
      return (
        <div class="mr-2 text-truegray-5">
          {isDir
            ? <FolderFilled />
            : <FileFilled />}
        </div>
      )
    }

    return (
      <div class="flex">
        {renderIcon(args.record.is_dir)}
        {args.text}
      </div>
    )
  },
  width: 500,
}, {
  title: () => $gettext('Updated at'),
  dataIndex: 'modified_at',
  customRender: datetime,
  datetime: true,
  sorter: true,
  pithy: true,
  width: 200,
}, {
  title: () => $gettext('Action'),
  dataIndex: 'action',
  fixed: 'right',
  width: 180,
}]

export default configColumns
