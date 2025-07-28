import type { StdTableColumn } from '@uozi-admin/curd'
import { datetimeRender } from '@uozi-admin/curd'
import { PostSyncAction } from '@/api/env_group'
import { useNodeAvailabilityStore } from '@/pinia/moudule/nodeAvailability'

const columns: StdTableColumn[] = [{
  dataIndex: 'name',
  title: () => $gettext('Name'),
  search: true,
  edit: {
    type: 'input',
  },
  pure: true,
  width: 120,
}, {
  title: () => $gettext('Sync Nodes'),
  dataIndex: 'sync_node_ids',
  customRender: ({ text }) => {
    const nodeStore = useNodeAvailabilityStore()

    if (!text || text.length === 0) {
      return h('span', { class: 'text-gray-400' }, '-')
    }

    const nodeElements = text.map((nodeId: number) => {
      const nodeStatus = nodeStore.getNodeStatus(nodeId)
      const nodeName = nodeStatus?.name || `Node ${nodeId}`
      const isOnline = nodeStatus?.status ?? false

      return h('div', {
        class: 'inline-flex items-center mr-2 mb-1',
      }, [
        h('span', {
          class: `inline-block w-2 h-2 rounded-full mr-1 flex-shrink-0 ${isOnline ? 'bg-green-500' : 'bg-red-500'}`,
        }),
        h('span', nodeName),
      ])
    })

    return h('div', { class: 'flex flex-wrap' }, nodeElements)
  },
  pure: true,
  width: 200,
}, {
  title: () => $gettext('Post-sync Action'),
  dataIndex: 'post_sync_action',
  customRender: ({ text }) => {
    if (!text || text === PostSyncAction.None) {
      return $gettext('No Action')
    }
    else if (text === PostSyncAction.ReloadNginx) {
      return $gettext('Reload Nginx')
    }
    return text
  },
  pure: true,
  width: 150,
}, {
  title: () => $gettext('Created at'),
  dataIndex: 'created_at',
  customRender: datetimeRender,
  pure: true,
  width: 150,
}, {
  title: () => $gettext('Updated at'),
  dataIndex: 'updated_at',
  customRender: datetimeRender,
  pure: true,
  width: 150,
}, {
  title: () => $gettext('Actions'),
  dataIndex: 'actions',
  fixed: 'right',
  width: 150,
}]

export default columns
