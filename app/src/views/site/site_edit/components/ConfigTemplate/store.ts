import type { Template } from '@/api/template'
import template from '@/api/template'
import { debounce } from 'lodash'
import { defineStore } from 'pinia'

export const useConfigTemplateStore = defineStore('configTemplate', () => {
  const data = ref<Template>({} as Template)

  const variables = computed(() => data.value?.variables ?? {})

  function __buildTemplate() {
    template.build_block(data.value.filename, variables.value).then(r => {
      data.value.directives = r.directives
      data.value.locations = r.locations
      data.value.custom = r.custom
    })
  }

  const buildTemplate = debounce(__buildTemplate, 500)

  return {
    data,
    variables,
    buildTemplate,
  }
})
