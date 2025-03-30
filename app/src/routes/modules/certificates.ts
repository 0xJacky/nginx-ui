import type { RouteRecordRaw } from 'vue-router'
import { SafetyCertificateOutlined } from '@ant-design/icons-vue'

export const certificatesRoutes: RouteRecordRaw[] = [
  {
    path: 'certificates',
    name: 'Certificates',
    component: () => import('@/layouts/BaseRouterView.vue'),
    redirect: '/certificates/list',
    meta: {
      name: () => $gettext('Certificates'),
      icon: SafetyCertificateOutlined,
    },
    children: [
      {
        path: 'acme_users',
        name: 'ACME User',
        component: () => import('@/views/certificate/ACMEUser.vue'),
        meta: {
          name: () => $gettext('ACME User'),
        },
      },
      {
        path: 'list',
        name: 'Certificates List',
        component: () => import('@/views/certificate/CertificateList/Certificate.vue'),
        meta: {
          name: () => $gettext('Certificates List'),
        },
      },
      {
        path: ':id',
        name: 'Modify Certificate',
        component: () => import('@/views/certificate/CertificateEditor.vue'),
        meta: {
          name: () => $gettext('Modify Certificate'),
          hiddenInSidebar: true,
          lastRouteName: 'Certificates List',
        },
      },
      {
        path: 'import',
        name: 'Import Certificate',
        component: () => import('@/views/certificate/CertificateEditor.vue'),
        meta: {
          name: () => $gettext('Import Certificate'),
          hiddenInSidebar: true,
          lastRouteName: 'Certificates List',
        },
      },
      {
        path: 'dns_credential',
        name: 'DNS Credentials',
        component: () => import('@/views/certificate/DNSCredential.vue'),
        meta: {
          name: () => $gettext('DNS Credentials'),
        },
      },
    ],
  },
]
