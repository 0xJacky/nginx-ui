import type { RouteRecordRaw } from 'vue-router'
import { useSettingsStore, useUserStore } from '@/pinia'

import {
  BellOutlined,
  CloudOutlined,
  CodeOutlined,
  DatabaseOutlined,
  FileOutlined,
  FileTextOutlined,
  HomeOutlined,
  InfoCircleOutlined,
  SafetyCertificateOutlined,
  SettingOutlined,
  ShareAltOutlined,
  UserOutlined,
} from '@ant-design/icons-vue'
import NProgress from 'nprogress'

import { createRouter, createWebHashHistory } from 'vue-router'

import 'nprogress/nprogress.css'

export const routes: RouteRecordRaw[] = [
  {
    path: '/',
    name: 'Home',
    component: () => import('@/layouts/BaseLayout.vue'),
    redirect: '/dashboard',
    meta: {
      name: () => $gettext('Home'),
    },
    children: [
      {
        path: 'dashboard',
        component: () => import('@/views/dashboard/DashBoard.vue'),
        name: 'Dashboard',
        meta: {
          name: () => $gettext('Dashboard'),
          icon: HomeOutlined,
        },
      },
      {
        path: 'sites',
        name: 'Manage Sites',
        component: () => import('@/layouts/BaseRouterView.vue'),
        meta: {
          name: () => $gettext('Manage Sites'),
          icon: CloudOutlined,
        },
        redirect: '/sites/list',
        children: [{
          path: 'list',
          name: 'Sites List',
          component: () => import('@/views/site/SiteList.vue'),
          meta: {
            name: () => $gettext('Sites List'),
          },
        }, {
          path: 'add',
          name: 'Add Site',
          component: () => import('@/views/site/SiteAdd.vue'),
          meta: {
            name: () => $gettext('Add Site'),
            lastRouteName: 'Sites List',
          },
        }, {
          path: 'categories',
          name: 'Site Categories',
          component: () => import('@/views/site/site_category/SiteCategory.vue'),
          meta: {
            name: () => $gettext('Site Categories'),
          },
        }, {
          path: ':name',
          name: 'Edit Site',
          component: () => import('@/views/site/SiteEdit.vue'),
          meta: {
            name: () => $gettext('Edit Site'),
            hiddenInSidebar: true,
            lastRouteName: 'Sites List',
          },
        }],
      },
      {
        path: 'streams',
        name: 'Manage Streams',
        component: () => import('@/views/stream/StreamList.vue'),
        meta: {
          name: () => $gettext('Manage Streams'),
          icon: ShareAltOutlined,
        },
      },
      {
        path: 'stream/:name',
        name: 'Edit Stream',
        component: () => import('@/views/stream/StreamEdit.vue'),
        meta: {
          name: () => $gettext('Edit Stream'),
          hiddenInSidebar: true,
          lastRouteName: 'Manage Streams',
        },
      },
      {
        path: 'config',
        name: 'Manage Configs',
        component: () => import('@/views/config/ConfigList.vue'),
        meta: {
          name: () => $gettext('Manage Configs'),
          icon: FileOutlined,
          hideChildren: true,
        },
      },
      {
        path: 'config/add',
        name: 'Add Configuration',
        component: () => import('@/views/config/ConfigEditor.vue'),
        meta: {
          name: () => $gettext('Add Configuration'),
          hiddenInSidebar: true,
          lastRouteName: 'Manage Configs',
        },
      },
      {
        path: 'config/:name+/edit',
        name: 'Edit Configuration',
        component: () => import('@/views/config/ConfigEditor.vue'),
        meta: {
          name: () => $gettext('Edit Configuration'),
          hiddenInSidebar: true,
          lastRouteName: 'Manage Configs',
        },
      },
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
      {
        path: 'terminal',
        name: 'Terminal',
        component: () => import('@/views/terminal/Terminal.vue'),
        meta: {
          name: () => $gettext('Terminal'),
          icon: CodeOutlined,
        },
      },
      {
        path: 'nginx_log',
        name: 'Nginx Log',
        meta: {
          name: () => $gettext('Nginx Log'),
          icon: FileTextOutlined,
        },
        children: [{
          path: 'access',
          name: 'Access Logs',
          component: () => import('@/views/nginx_log/NginxLog.vue'),
          meta: {
            name: () => $gettext('Access Logs'),
          },
        }, {
          path: 'error',
          name: 'Error Logs',
          component: () => import('@/views/nginx_log/NginxLog.vue'),
          meta: {
            name: () => $gettext('Error Logs'),
          },
        }, {
          path: 'site',
          name: 'Site Logs',
          component: () => import('@/views/nginx_log/NginxLog.vue'),
          meta: {
            name: () => $gettext('Site Logs'),
            hiddenInSidebar: true,
          },
        }],
      },
      {
        path: 'environment',
        name: 'Environment',
        component: () => import('@/views/environment/Environment.vue'),
        meta: {
          name: () => $gettext('Environment'),
          icon: DatabaseOutlined,
          hiddenInSidebar: (): boolean => {
            const settings = useSettingsStore()

            return settings.is_remote
          },
        },
      },
      {
        path: 'notifications',
        name: 'Notifications',
        component: () => import('@/views/notification/Notification.vue'),
        meta: {
          name: () => $gettext('Notifications'),
          icon: BellOutlined,
        },
      },
      {
        path: 'user',
        name: 'Manage Users',
        component: () => import('@/views/user/User.vue'),
        meta: {
          name: () => $gettext('Manage Users'),
          icon: UserOutlined,
        },
      },
      {
        path: 'preference',
        name: 'Preference',
        component: () => import('@/views/preference/Preference.vue'),
        meta: {
          name: () => $gettext('Preference'),
          icon: SettingOutlined,
        },
      },
      {
        path: 'system',
        name: 'System',
        redirect: 'system/about',
        meta: {
          name: () => $gettext('System'),
          icon: InfoCircleOutlined,
        },
        children: [{
          path: 'about',
          name: 'About',
          component: () => import('@/views/system/About.vue'),
          meta: {
            name: () => $gettext('About'),
          },
        }, {
          path: 'upgrade',
          name: 'Upgrade',
          component: () => import('@/views/system/Upgrade.vue'),
          meta: {
            name: () => $gettext('Upgrade'),
            hiddenInSidebar: (): boolean => {
              const settings = useSettingsStore()

              return settings.is_remote
            },
          },
        }],
      },
    ],
  },
  {
    path: '/install',
    name: 'Install',
    component: () => import('@/views/other/Install.vue'),
    meta: { name: () => $gettext('Install'), noAuth: true },
  },
  {
    path: '/login',
    name: 'Login',
    component: () => import('@/views/other/Login.vue'),
    meta: { name: () => $gettext('Login'), noAuth: true },
  },
  {
    path: '/:pathMatch(.*)*',
    name: 'Not Found',
    component: () => import('@/views/other/Error.vue'),
    meta: { name: () => $gettext('Not Found'), noAuth: true, status_code: 404, error: () => $gettext('Not Found') },
  },
]

const router = createRouter({
  history: createWebHashHistory(),
  routes,
})

NProgress.configure({ showSpinner: false })

router.beforeEach((to, _, next) => {
  document.title = `${to?.meta.name?.() ?? ''} | Nginx UI`

  NProgress.start()

  const user = useUserStore()

  if (to.meta.noAuth || user.isLogin)
    next()
  else
    next({ path: '/login', query: { next: to.fullPath } })
})

router.afterEach(() => {
  NProgress.done()
})

export default router
