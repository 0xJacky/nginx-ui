import type { RouteRecordRaw } from 'vue-router'
import { createRouter, createWebHashHistory } from 'vue-router'
import { useNProgress } from '@/lib/nprogress/nprogress'
import { useUserStore } from '@/pinia'
import { authRoutes } from './modules/auth'

import { backupRoutes } from './modules/backup'
import { certificatesRoutes } from './modules/certificates'
import { configRoutes } from './modules/config'
import { dashboardRoutes } from './modules/dashboard'
import { errorRoutes } from './modules/error'
import { namespacesRoutes } from './modules/namespaces'
import { nginxLogRoutes } from './modules/nginx_log'
import { nodesRoutes } from './modules/nodes'
import { notificationsRoutes } from './modules/notifications'
import { preferenceRoutes } from './modules/preference'
import { sitesRoutes } from './modules/sites'
import { streamsRoutes } from './modules/streams'
import { systemRoutes } from './modules/system'
import { terminalRoutes } from './modules/terminal'
import { upstreamRoutes } from './modules/upstream'
import { userRoutes } from './modules/user'
import 'nprogress/nprogress.css'

// Combine child routes for the main layout
const mainLayoutChildren: RouteRecordRaw[] = [
  ...dashboardRoutes,
  ...sitesRoutes,
  ...streamsRoutes,
  ...upstreamRoutes,
  ...configRoutes,
  ...certificatesRoutes,
  ...terminalRoutes,
  ...nginxLogRoutes,
  ...namespacesRoutes,
  ...nodesRoutes,
  ...notificationsRoutes,
  ...userRoutes,
  ...preferenceRoutes,
  ...backupRoutes,
  ...systemRoutes,
]

// Main routes configuration
export const routes: RouteRecordRaw[] = [
  {
    path: '/',
    name: 'Home',
    component: () => import('@/layouts/BaseLayout.vue'),
    redirect: '/dashboard',
    meta: {
      name: () => $gettext('Home'),
    },
    children: mainLayoutChildren,
  },
  {
    path: '/workspace',
    name: 'Workspace',
    component: () => import('@/views/workspace/WorkSpace.vue'),
    meta: {
      name: () => $gettext('Workspace'),
    },
  },
  ...authRoutes,
  ...errorRoutes,
]

const router = createRouter({
  history: createWebHashHistory(),
  routes,
})

const nprogress = useNProgress()

router.beforeEach((to, _, next) => {
  document.title = `${to?.meta.name?.() ?? ''} | Nginx UI`

  nprogress.start()

  const user = useUserStore()

  if (to.meta.noAuth || user.isLogin)
    next()
  else
    next({ path: '/login', query: { next: to.fullPath } })
})

router.afterEach(() => {
  nprogress.done()
})

export default router
