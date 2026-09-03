import type { RouteRecordRaw } from 'vue-router'
import { ClusterOutlined } from '@antdv-next/icons'

export const upstreamRoutes: RouteRecordRaw[] = [
  {
    path: 'upstream',
    name: 'Upstream Management',
    component: () => import('@/views/upstream/SocketList.vue'),
    meta: {
      name: () => $gettext('Upstream'),
      icon: ClusterOutlined,
    },
  },
]
