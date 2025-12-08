import type { RouteRecordRaw } from 'vue-router'
import { CloudServerOutlined } from '@ant-design/icons-vue'

export const dnsRoutes: RouteRecordRaw[] = [
  {
    path: 'dns',
    name: 'DNS',
    component: () => import('@/layouts/BaseRouterView.vue'),
    redirect: '/dns/domains',
    meta: {
      name: () => $gettext('DNS'),
      icon: CloudServerOutlined,
    },
    children: [
      {
        path: 'credentials',
        name: 'DNS Credentials',
        component: () => import('@/views/dns/DNSCredential.vue'),
        meta: {
          name: () => $gettext('Credentials'),
        },
      },
      {
        path: 'domains',
        name: 'DNS Domains',
        component: () => import('@/views/dns/DNSDomainList.vue'),
        meta: {
          name: () => $gettext('DNS Domains'),
        },
      },
      {
        path: 'ddns',
        name: 'DNS DDNS',
        component: () => import('@/views/dns/DDNSManager.vue'),
        meta: {
          name: () => $gettext('DDNS'),
        },
      },
      {
        path: 'domains/:id/records',
        name: 'DNS Domain Records',
        component: () => import('@/views/dns/DNSRecordManager.vue'),
        meta: {
          name: () => $gettext('DNS Records'),
          hiddenInSidebar: true,
          lastRouteName: 'DNS Domains',
        },
      },
    ],
  },
]
