import type { RouteRecordRaw } from 'vue-router'
import { HomeOutlined } from '@antdv-next/icons'

export const dashboardRoutes: RouteRecordRaw[] = [
  {
    path: 'dashboard',
    component: () => import('@/layouts/BaseRouterView.vue'),
    name: 'Dashboard',
    meta: {
      name: () => $gettext('Dashboard'),
      icon: HomeOutlined,
    },
    children: [
      {
        path: '',
        component: () => import('@/views/dashboard/index.vue'),
        name: 'Dashboard Home',
        meta: {
          name: () => $gettext('Dashboard'),
          hiddenInSidebar: true,
        },
      },
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
