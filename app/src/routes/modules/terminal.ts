import type { RouteRecordRaw } from 'vue-router'
import { CodeOutlined } from '@antdv-next/icons'

export const terminalRoutes: RouteRecordRaw[] = [
  {
    path: 'terminal',
    name: 'Terminal',
    component: () => import('@/views/terminal/Terminal.vue'),
    meta: {
      name: () => $gettext('Terminal'),
      icon: CodeOutlined,
    },
  },
]
