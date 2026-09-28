import type { RouteRecordRaw } from 'vue-router'
import { ClusterOutlined } from '@antdv-next/icons'

export const upstreamRoutes: RouteRecordRaw[] = [
  {
    path: 'upstream',
    name: 'Upstream Management',
    component: () => import('@/layouts/BaseRouterView.vue'),
    meta: {
      name: () => $gettext('Upstream'),
      icon: ClusterOutlined,
    },
    children: [
      {
        path: '',
        name: 'Upstream Home',
        component: () => import('@/views/upstream/index.vue'),
        meta: {
          name: () => $gettext('Upstream'),
          hiddenInSidebar: true,
        },
      },
      {
        path: 'list',
        name: 'Upstream List',
        component: () => import('@/views/upstream/UpstreamList.vue'),
        meta: {
          name: () => $gettext('Upstream Groups'),
        },
      },
      {
        path: 'sockets',
        name: 'Upstream Sockets',
        component: () => import('@/views/upstream/SocketList.vue'),
        meta: {
          name: () => $gettext('Upstream Sockets'),
        },
      },
    ],
  },
]
