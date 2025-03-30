import type { RouteRecordRaw } from 'vue-router'
import { useNProgress } from '@/lib/nprogress/nprogress'
import { useUserStore } from '@/pinia'
import { createRouter, createWebHashHistory } from 'vue-router'
import { authRoutes } from './modules/auth'

import { certificatesRoutes } from './modules/certificates'
import { configRoutes } from './modules/config'
// Import module routes
import { dashboardRoutes } from './modules/dashboard'
import { environmentsRoutes } from './modules/environments'
import { errorRoutes } from './modules/error'
import { nginxLogRoutes } from './modules/nginx_log'
import { notificationsRoutes } from './modules/notifications'
import { preferenceRoutes } from './modules/preference'
import { sitesRoutes } from './modules/sites'
import { streamsRoutes } from './modules/streams'
import { systemRoutes } from './modules/system'
import { terminalRoutes } from './modules/terminal'
import { userRoutes } from './modules/user'
import 'nprogress/nprogress.css'

// Combine child routes for the main layout
const mainLayoutChildren: RouteRecordRaw[] = [
  ...dashboardRoutes,
  ...sitesRoutes,
  ...streamsRoutes,
  ...configRoutes,
  ...certificatesRoutes,
  ...terminalRoutes,
  ...nginxLogRoutes,
  ...environmentsRoutes,
  ...notificationsRoutes,
  ...userRoutes,
  ...preferenceRoutes,
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
