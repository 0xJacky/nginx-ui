import { createRouter, createWebHashHistory } from 'vue-router'
import type { AntDesignOutlinedIconType } from '@ant-design/icons-vue/lib/icons/AntDesignOutlined'

import {
  CloudOutlined,
  CodeOutlined,
  DatabaseOutlined,
  FileOutlined,
  FileTextOutlined,
  HomeOutlined,
  InfoCircleOutlined,
  SafetyCertificateOutlined,
  SettingOutlined,
  UserOutlined,
} from '@ant-design/icons-vue'
import NProgress from 'nprogress'

import gettext from '../gettext'
import { useUserStore } from '@/pinia'

import 'nprogress/nprogress.css'

const { $gettext } = gettext

export interface Route {
  path: string
  name: () => string
  component?: () => Promise<typeof import('*.vue')>
  redirect?: string
  meta?: {
    icon?: AntDesignOutlinedIconType
    hiddenInSidebar?: boolean
    hideChildren?: boolean
    noAuth?: boolean
    status_code?: number
    error?: () => string
  }
  children?: Route[]
}

export const routes: Route[] = [
  {
    path: '/',
    name: () => $gettext('Home'),
    component: () => import('@/layouts/BaseLayout.vue'),
    redirect: '/dashboard',
    children: [
      {
        path: 'dashboard',
        component: () => import('@/views/dashboard/DashBoard.vue'),
        name: () => $gettext('Dashboard'),
        meta: {
          icon: HomeOutlined,
        },
      },
      {
        path: 'domain',
        name: () => $gettext('Manage Sites'),
        component: () => import('@/layouts/BaseRouterView.vue'),
        meta: {
          icon: CloudOutlined,
        },
        redirect: '/domain/list',
        children: [{
          path: 'list',
          name: () => $gettext('Sites List'),
          component: () => import('@/views/domain/DomainList.vue'),
        }, {
          path: 'add',
          name: () => $gettext('Add Site'),
          component: () => import('@/views/domain/DomainAdd.vue'),
        }, {
          path: ':name',
          name: () => $gettext('Edit Site'),
          component: () => import('@/views/domain/DomainEdit.vue'),
          meta: {
            hiddenInSidebar: true,
          },
        }],
      },
      {
        path: 'config',
        name: () => $gettext('Manage Configs'),
        component: () => import('@/views/config/Config.vue'),
        meta: {
          icon: FileOutlined,
          hideChildren: true,
        },
      },
      {
        path: 'config/:name+/edit',
        name: () => $gettext('Edit Configuration'),
        component: () => import('@/views/config/ConfigEdit.vue'),
        meta: {
          hiddenInSidebar: true,
        },
      },
      {
        path: 'cert',
        name: () => $gettext('Certification'),
        component: () => import('@/layouts/BaseRouterView.vue'),
        meta: {
          icon: SafetyCertificateOutlined,
        },
        children: [
          {
            path: 'list',
            name: () => $gettext('Certification List'),
            component: () => import('@/views/cert/Cert.vue'),
          },
          {
            path: 'dns_credential',
            name: () => $gettext('DNS Credentials'),
            component: () => import('@/views/cert/DNSCredential.vue'),
          },
        ],
      },
      {
        path: 'terminal',
        name: () => $gettext('Terminal'),
        component: () => import('@/views/pty/Terminal.vue'),
        meta: {
          icon: CodeOutlined,
        },
      },
      {
        path: 'nginx_log',
        name: () => $gettext('Nginx Log'),
        meta: {
          icon: FileTextOutlined,
        },
        children: [{
          path: 'access',
          name: () => $gettext('Access Logs'),
          component: () => import('@/views/nginx_log/NginxLog.vue'),
        }, {
          path: 'error',
          name: () => $gettext('Error Logs'),
          component: () => import('@/views/nginx_log/NginxLog.vue'),
        }, {
          path: 'site',
          name: () => $gettext('Site Logs'),
          component: () => import('@/views/nginx_log/NginxLog.vue'),
          meta: {
            hiddenInSidebar: true,
          },
        }],
      },
      {
        path: 'environment',
        name: () => $gettext('Environment'),
        component: () => import('@/views/environment/Environment.vue'),
        meta: {
          icon: DatabaseOutlined,
        },
      },
      {
        path: 'user',
        name: () => $gettext('Manage Users'),
        component: () => import('@/views/user/User.vue'),
        meta: {
          icon: UserOutlined,
        },
      },
      {
        path: 'preference',
        name: () => $gettext('Preference'),
        component: () => import('@/views/preference/Preference.vue'),
        meta: {
          icon: SettingOutlined,
        },
      },
      {
        path: 'system',
        name: () => $gettext('System'),
        redirect: 'system/about',
        meta: {
          icon: InfoCircleOutlined,
        },
        children: [{
          path: 'about',
          name: () => $gettext('About'),
          component: () => import('@/views/system/About.vue'),
        }, {
          path: 'upgrade',
          name: () => $gettext('Upgrade'),
          component: () => import('@/views/system/Upgrade.vue'),
        }],
      },
    ],
  },
  {
    path: '/install',
    name: () => $gettext('Install'),
    component: () => import('@/views/other/Install.vue'),
    meta: { noAuth: true },
  },
  {
    path: '/login',
    name: () => $gettext('Login'),
    component: () => import('@/views/other/Login.vue'),
    meta: { noAuth: true },
  },
  {
    path: '/:pathMatch(.*)*',
    name: () => $gettext('Not Found'),
    component: () => import('@/views/other/Error.vue'),
    meta: { noAuth: true, status_code: 404, error: () => $gettext('Not Found') },
  },
]

const router = createRouter({
  history: createWebHashHistory(),

  // @ts-expect-error routes type error
  routes,
})

NProgress.configure({ showSpinner: false })

router.beforeEach((to, _, next) => {
  // @ts-expect-error name type
  document.title = `${to.name?.()} | Nginx UI`

  NProgress.start()

  const user = useUserStore()
  const { is_login } = user

  if (to.meta.noAuth || is_login)
    next()
  else
    next({ path: '/login', query: { next: to.fullPath } })
})

router.afterEach(() => {
  NProgress.done()
})

export default router
