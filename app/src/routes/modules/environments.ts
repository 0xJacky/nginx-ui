import type { RouteRecordRaw } from 'vue-router'
import { useSettingsStore } from '@/pinia'
import { DatabaseOutlined } from '@ant-design/icons-vue'

export const environmentsRoutes: RouteRecordRaw[] = [
  {
    path: 'environments',
    name: 'Environments',
    component: () => import('@/views/environment/Environment.vue'),
    meta: {
      name: () => $gettext('Environments'),
      icon: DatabaseOutlined,
      hiddenInSidebar: (): boolean => {
        const settings = useSettingsStore()

        return settings.is_remote
      },
    },
  },
]
