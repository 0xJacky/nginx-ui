import type { RouteRecordRaw } from 'vue-router'

export const authRoutes: RouteRecordRaw[] = [
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
]
