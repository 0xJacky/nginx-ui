import type { DirectiveMap } from '@/api/ngx'
import ngx from '@/api/ngx'

export const useDirectiveStore = defineStore('directive', () => {
  const curIdx = ref(-1)
  const nginxDirectivesDocsMap = ref<DirectiveMap>()
  const nginxDirectivesOptions = ref<{ label: string, value: string }[]>([])

  async function getNginxDirectivesDocsMap() {
    nginxDirectivesDocsMap.value = await ngx.get_directives()
    await nextTick()
    nginxDirectivesOptions.value = Object.keys(nginxDirectivesDocsMap.value).map(k => ({ label: k, value: k }))
  }

  return {
    curIdx,
    nginxDirectivesDocsMap,
    getNginxDirectivesDocsMap,
    nginxDirectivesOptions,
  }
})
