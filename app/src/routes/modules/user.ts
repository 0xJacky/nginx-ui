import type { RouteRecordRaw } from 'vue-router'
import { UsergroupAddOutlined, UserOutlined } from '@ant-design/icons-vue'

export const userRoutes: RouteRecordRaw[] = [
  {
    path: 'users',
    name: 'Manage Users',
    component: () => import('@/views/user/User.vue'),
    meta: {
      name: () => $gettext('Manage Users'),
      icon: UsergroupAddOutlined,
    },
  },
  {
    path: 'profile',
    name: 'User Profile',
    component: () => import('@/views/user/UserProfile.vue'),
    meta: {
      name: () => $gettext('User Profile'),
      icon: UserOutlined,
    },
  },
]
