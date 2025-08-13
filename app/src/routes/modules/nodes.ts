import type { RouteRecordRaw } from 'vue-router'
import { DatabaseOutlined } from '@ant-design/icons-vue'
import { useSettingsStore } from '@/pinia'

export const nodesRoutes: RouteRecordRaw[] = [
  {
    path: 'nodes',
    name: 'Nodes',
    component: () => import('@/views/node/Node.vue'),
    meta: {
      name: () => $gettext('Nodes'),
      icon: DatabaseOutlined,
      hiddenInSidebar: (): boolean => {
        const settings = useSettingsStore()

        return settings.is_remote
      },
    },
  },
]
