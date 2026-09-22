import type { RouteRecordRaw } from 'vue-router'
import { StopOutlined } from '@antdv-next/icons'

export const securityRoutes: RouteRecordRaw[] = [
  {
    path: 'blocklists',
    name: 'Blocklists',
    component: () => import('@/views/security/BlocklistSources.vue'),
    meta: {
      name: () => $gettext('Blocklists'),
      icon: StopOutlined,
    },
  },
]
