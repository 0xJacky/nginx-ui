import type { RouteRecordRaw } from 'vue-router'

export const errorRoutes: RouteRecordRaw[] = [
  {
    path: '/:pathMatch(.*)*',
    name: 'Not Found',
    component: () => import('@/views/other/Error.vue'),
    meta: { name: () => $gettext('Not Found'), noAuth: true, status_code: 404, error: () => $gettext('Not Found') },
  },
]
