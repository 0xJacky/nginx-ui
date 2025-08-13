import type { RouteRecordRaw } from 'vue-router'
import { StarOutlined } from '@ant-design/icons-vue'

export const namespacesRoutes: RouteRecordRaw[] = [
  {
    path: 'namespaces',
    name: 'Namespaces',
    component: () => import('@/views/namespace/Namespace.vue'),
    meta: {
      name: () => $gettext('Namespaces'),
      icon: StarOutlined,
    },
  },
]
