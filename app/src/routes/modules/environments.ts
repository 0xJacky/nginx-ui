import type { RouteRecordRaw } from 'vue-router'
import { DatabaseOutlined } from '@ant-design/icons-vue'
import { useSettingsStore } from '@/pinia'

export const environmentsRoutes: RouteRecordRaw[] = [
  {
    path: 'environments',
    name: 'Environments',
    component: () => import('@/layouts/BaseRouterView.vue'),
    meta: {
      name: () => $gettext('Environments'),
      icon: DatabaseOutlined,
      hiddenInSidebar: (): boolean => {
        const settings = useSettingsStore()

        return settings.is_remote
      },
    },
    children: [
      {
        path: 'list',
        name: 'env.list',
        component: () => import('@/views/environments/list/Environment.vue'),
        meta: {
          name: () => $gettext('Nodes'),
        },
      },
      {
        path: 'groups',
        name: 'env.groups',
        component: () => import('@/views/environments/group/EnvGroup.vue'),
        meta: {
          name: () => $gettext('Node Groups'),
        },
      },
    ],
  },
]
