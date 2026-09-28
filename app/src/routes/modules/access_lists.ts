import type { RouteRecordRaw } from 'vue-router'
import { SafetyOutlined } from '@antdv-next/icons'

export const accessListsRoutes: RouteRecordRaw[] = [
  {
    path: 'access-lists',
    name: 'Access Lists',
    component: () => import('@/views/access_list/AccessListList.vue'),
    meta: {
      name: () => $gettext('Access Lists'),
      icon: SafetyOutlined,
    },
  },
]
