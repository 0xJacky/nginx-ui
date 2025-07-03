import type { NgxModule } from '@/api/ngx'
import type { NginxStatus } from '@/constants'
import { defineStore } from 'pinia'

interface ProcessingStatus {
  index_scanning: boolean
  auto_cert_processing: boolean
}

interface NginxLogStatus {
  scanning: boolean
}

type NginxStatusType = NginxStatus.Reloading | NginxStatus.Restarting | NginxStatus.Running | NginxStatus.Stopped

export const useGlobalStore = defineStore('global', () => {
  const nginxStatus: Ref<NginxStatusType> = ref(0)

  const processingStatus = ref<ProcessingStatus>({
    index_scanning: false,
    auto_cert_processing: false,
  })

  const nginxLogStatus = ref<NginxLogStatus>({
    scanning: false,
  })

  const modules = ref<NgxModule[]>([])
  const modulesMap = ref<Record<string, NgxModule>>({})

  return {
    nginxStatus,
    processingStatus,
    nginxLogStatus,
    modules,
    modulesMap,
  }
})
