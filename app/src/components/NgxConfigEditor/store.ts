import type { NgxConfig, NgxDirective } from '@/api/ngx'
import { defineStore } from 'pinia'

export const useNgxConfigStore = defineStore('ngxConfig', () => {
  const ngxConfig = ref<NgxConfig>({
    name: '',
    servers: [],
    upstreams: [],
  })

  const configText = ref('')

  const curServerIdx = ref(0)

  function setNgxConfig(config: NgxConfig) {
    ngxConfig.value = config
  }

  const curServer = computed({
    get() {
      return ngxConfig.value.servers[curServerIdx.value]
    },
    set(v) {
      ngxConfig.value.servers[curServerIdx.value] = v
    },
  })

  const curServerDirectives = computed({
    get() {
      return ngxConfig.value.servers[curServerIdx.value]?.directives
    },
    set(v) {
      ngxConfig.value.servers[curServerIdx.value].directives = v
    },
  })

  const curServerLocations = computed({
    get() {
      return ngxConfig.value.servers[curServerIdx.value]?.locations
    },
    set(v) {
      ngxConfig.value.servers[curServerIdx.value].locations = v
    },
  })

  const curDirectivesMap = computed(() => {
    const record: Record<string, NgxDirective[]> = {}

    curServerDirectives.value?.forEach((v, k) => {
      v.idx = k
      if (record[v.directive])
        record[v.directive].push(v)
      else
        record[v.directive] = [v]
    })

    return record
  })

  return {
    ngxConfig,
    configText,
    curServerIdx,
    setNgxConfig,
    curServer,
    curServerDirectives,
    curServerLocations,
    curDirectivesMap,
  }
})
