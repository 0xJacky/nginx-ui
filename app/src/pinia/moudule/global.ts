import { defineStore } from 'pinia'
import type { NginxStatus } from '@/constants'

export const useGlobalStore = defineStore('global', () => {
  const nginxStatus:
  Ref<NginxStatus.Reloading | NginxStatus.Restarting | NginxStatus.Running | NginxStatus.Stopped>
      = ref(0)

  return {
    nginxStatus,
  }
})
