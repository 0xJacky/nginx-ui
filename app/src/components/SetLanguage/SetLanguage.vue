<script setup lang="ts">
import { ref, watch } from 'vue'

import { useSettingsStore } from '@/pinia'
import http from '@/lib/http'
import gettext from '@/gettext'

const settings = useSettingsStore()

const route = useRoute()

const current = ref(gettext.current)

const languageAvailable = gettext.available

async function init() {
  if (current.value !== 'en') {
    await http.get(`/translation/${current.value}`).then(r => {
      gettext.translations[current.value] = r
    })

    document.title = `${route.meta.name?.()} | Nginx UI`
  }
}

init()

watch(current, v => {
  init()
  settings.set_language(v)
  gettext.current = v

  const name = route.meta.name as never as () => string

  document.title = `${name()} | Nginx UI`
})

</script>

<template>
  <div>
    <ASelect
      v-model:value="current"
      size="small"
      style="width: 60px"
    >
      <ASelectOption
        v-for="(language, key) in languageAvailable"
        :key="key"
        :value="key"
      >
        {{ language }}
      </ASelectOption>
    </ASelect>
  </div>
</template>

<style lang="less" scoped>

</style>
