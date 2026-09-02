import type { RouteRecordRaw } from 'vue-router'
import { ShareAltOutlined } from '@antdv-next/icons'

export const streamsRoutes: RouteRecordRaw[] = [
  {
    path: 'streams',
    name: 'Manage Streams',
    component: () => import('@/views/stream/StreamList.vue'),
    meta: {
      name: () => $gettext('Manage Streams'),
      icon: ShareAltOutlined,
      modules: ['stream'],
    },
  },
  {
    path: 'streams/:name',
    name: 'Edit Stream',
    component: () => import('@/views/stream/StreamEdit.vue'),
    meta: {
      name: () => $gettext('Edit Stream'),
      hiddenInSidebar: true,
      lastRouteName: 'Manage Streams',
      modules: ['stream'],
    },
  },
]
