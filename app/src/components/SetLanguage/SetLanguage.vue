<script setup lang="ts">
import { watch } from 'vue'

import { useSettingsStore } from '@/pinia'
import gettext from '@/gettext'
import loadTranslations from '@/api/translations'

const settings = useSettingsStore()

const route = useRoute()

const current = computed({
  get() {
    return gettext.current
  },
  set(v) {
    gettext.current = v
  },
})

const languageAvailable = gettext.available

watch(current, v => {
  loadTranslations()
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
