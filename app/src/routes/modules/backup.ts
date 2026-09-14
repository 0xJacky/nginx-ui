import type { RouteRecordRaw } from 'vue-router'
import { ClockCircleOutlined } from '@antdv-next/icons'

export const backupRoutes: RouteRecordRaw[] = [
  {
    path: 'backup',
    name: 'Backup',
    component: () => import('@/layouts/BaseRouterView.vue'),
    meta: {
      icon: ClockCircleOutlined,
      name: () => $gettext('Backup Management'),
    },
    children: [
      {
        path: '',
        name: 'Backup Home',
        component: () => import('@/views/backup/BackupHome.vue'),
        meta: {
          name: () => $gettext('Backup Management'),
          hiddenInSidebar: true,
        },
      },
      {
        path: 'backup-and-restore',
        name: 'BackupAndRestore',
        component: () => import('@/views/backup/index.vue'),
        meta: {
          name: () => $gettext('Backup'),
        },
      },
      {
        path: 'auto-backup',
        name: 'AutoBackup',
        component: () => import('@/views/backup/AutoBackup/AutoBackup.vue'),
        meta: {
          name: () => $gettext('Auto Backup'),
        },
      },
    ],
  },
]
