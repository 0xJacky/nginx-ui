import type { NgxConfig, NgxDirective } from '@/api/ngx'

function createEmptyNgxConfig(): NgxConfig {
  return {
    name: '',
    servers: [],
    upstreams: [],
  }
}

export const useNgxConfigStore = defineStore('ngxConfig', () => {
  const ngxConfig = ref<NgxConfig>(createEmptyNgxConfig())

  const configText = ref('')

  const curServerIdx = ref(0)

  function setNgxConfig(config: NgxConfig) {
    curServerIdx.value = 0
    ngxConfig.value = {
      ...createEmptyNgxConfig(),
      ...config,
    }
  }

  function reset() {
    curServerIdx.value = 0
    configText.value = ''
    ngxConfig.value = createEmptyNgxConfig()
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
    reset,
    curServer,
    curServerDirectives,
    curServerLocations,
    curDirectivesMap,
  }
})
