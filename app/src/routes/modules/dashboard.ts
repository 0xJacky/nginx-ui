import type { RouteRecordRaw } from 'vue-router'
import { HomeOutlined } from '@ant-design/icons-vue'

export const dashboardRoutes: RouteRecordRaw[] = [
  {
    path: 'dashboard',
    component: () => import('@/views/dashboard/DashBoard.vue'),
    name: 'Dashboard',
    meta: {
      name: () => $gettext('Dashboard'),
      icon: HomeOutlined,
    },
  },
]
