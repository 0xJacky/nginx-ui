import type { RouteLocationNormalized, RouteRecordRaw } from 'vue-router'
import { createRouter, createWebHashHistory } from 'vue-router'
import { useNProgress } from '@/lib/nprogress/nprogress'
import { useUserStore } from '@/pinia'
// The loader imports this module as well; both sides only use the other one
// inside functions, so the cycle is safe.
import { usePluginLoader, usePluginStore } from '@/plugin'
import { accessListsRoutes } from './modules/access_lists'
import { authRoutes } from './modules/auth'

import { backupRoutes } from './modules/backup'
import { certificatesRoutes } from './modules/certificates'
import { configRoutes } from './modules/config'
import { dashboardRoutes } from './modules/dashboard'
import { dnsRoutes } from './modules/dns'
import { errorRoutes } from './modules/error'
import { namespacesRoutes } from './modules/namespaces'
import { nginxLogRoutes } from './modules/nginx_log'
import { nodesRoutes } from './modules/nodes'
import { notificationsRoutes } from './modules/notifications'
import { preferenceRoutes } from './modules/preference'
import { securityRoutes } from './modules/security'
import { sitesRoutes } from './modules/sites'
import { streamsRoutes } from './modules/streams'
import { systemRoutes } from './modules/system'
import { terminalRoutes } from './modules/terminal'
import { upstreamRoutes } from './modules/upstream'
import { userRoutes } from './modules/user'
import 'nprogress/nprogress.css'

// Combine child routes for the main layout
const mainLayoutChildren: RouteRecordRaw[] = [
  {
    path: '',
    name: 'Home Overview',
    component: () => import('@/views/home/Home.vue'),
    meta: {
      name: () => $gettext('Home'),
      hiddenInSidebar: true,
    },
  },
  ...dashboardRoutes,
  ...sitesRoutes,
  ...streamsRoutes,
  ...upstreamRoutes,
  ...accessListsRoutes,
  ...securityRoutes,
  ...configRoutes,
  ...certificatesRoutes,
  ...dnsRoutes,
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

/** Name of the catch-all route declared in `modules/error.ts`. */
export const NOT_FOUND_ROUTE_NAME = 'Not Found'

// Main routes configuration
export const routes: RouteRecordRaw[] = [
  {
    path: '/',
    name: 'Home',
    component: () => import('@/layouts/BaseLayout.vue'),
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

/**
 * Plugin routes only exist after the loader ran, so opening a plugin URL
 * directly lands on the catch-all route first. Loads the plugins and returns
 * the URL to navigate to again when a real route now matches it.
 */
async function retryAfterPluginLoad(to: RouteLocationNormalized): Promise<string | undefined> {
  const pluginStore = usePluginStore()

  // Retry only while the first load is still ahead, which keeps the guard from looping.
  if (pluginStore.ready)
    return undefined

  await usePluginLoader().load()

  if (router.resolve(to.fullPath).name === NOT_FOUND_ROUTE_NAME)
    return undefined

  // A string keeps the query and the hash. The restarted navigation keeps the
  // push or replace of the original one, whose catch-all match never reached
  // the history.
  return to.fullPath
}

router.beforeEach(async to => {
  document.title = `${to?.meta.name?.() ?? ''} | Nginx UI`

  nprogress.start()

  const user = useUserStore()
  user.expireSession()

  if (to.name === NOT_FOUND_ROUTE_NAME && user.isLogin) {
    const retry = await retryAfterPluginLoad(to)
    if (retry)
      return retry
  }

  if (to.meta.noAuth || user.isLogin)
    return true

  return { path: '/login', query: { next: to.fullPath } }
})

router.afterEach(() => {
  nprogress.done()
})

export default router
