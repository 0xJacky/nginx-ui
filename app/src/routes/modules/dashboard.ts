import type { RouteRecordRaw } from 'vue-router'
import { HomeOutlined } from '@ant-design/icons-vue'

export const dashboardRoutes: RouteRecordRaw[] = [
  {
    path: 'dashboard',
    redirect: '/dashboard/server',
    name: 'Dashboard',
    meta: {
      name: () => $gettext('Dashboard'),
      icon: HomeOutlined,
    },
    children: [
      {
        path: 'server',
        component: () => import('@/views/dashboard/ServerDashBoard.vue'),
        name: 'Server',
        meta: {
          name: () => $gettext('Server'),
        },
      },
      {
        path: 'nginx',
        component: () => import('@/views/dashboard/NginxDashBoard.vue'),
        name: 'NginxPerformance',
        meta: {
          name: () => $gettext('Nginx'),
        },
      },
      {
        path: 'sites',
        component: () => import('@/views/dashboard/SiteNavigation.vue'),
        name: 'SiteNavigation',
        meta: {
          name: () => $gettext('Sites'),
        },
      },
    ],
  },
]
