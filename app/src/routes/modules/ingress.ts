import type { RouteRecordRaw } from 'vue-router'
import { ApiOutlined } from '@antdv-next/icons'

export const ingressRoutes: RouteRecordRaw[] = [
  {
    path: 'ingresses',
    name: 'Ingresses',
    component: () => import('@/views/ingress/IngressList.vue'),
    meta: {
      name: () => $gettext('Ingresses'),
      icon: ApiOutlined,
    },
  },
]
