import type { RouteRecordRaw } from 'vue-router'
import { UserOutlined } from '@ant-design/icons-vue'

export const userRoutes: RouteRecordRaw[] = [
  {
    path: 'user',
    name: 'Manage Users',
    component: () => import('@/views/user/User.vue'),
    meta: {
      name: () => $gettext('Manage Users'),
      icon: UserOutlined,
    },
  },
]
