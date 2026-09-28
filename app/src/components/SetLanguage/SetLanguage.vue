<script setup lang="ts">
import type { SelectProps } from 'antdv-next'
import loadTranslations from '@/api/translations'
import gettext from '@/gettext'
import { loadDayjsLocale } from '@/lib/helper/dayjsLocale'
import { useSettingsStore, useUserStore } from '@/pinia'

const settings = useSettingsStore()
const user = useUserStore()

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

const languageOptions = computed<SelectProps['options']>(() => Object.entries(languageAvailable).map(([key, language]) => ({
  label: language,
  value: key,
})))

function updateTitle() {
  const name = route.meta.name as never as () => string

  document.title = `${name()} | Nginx UI`
}

watch(current, v => {
  loadTranslations(route)
  settings.set_language(v)
  if (user.isLogin) {
    user.updateCurrentUserLanguage(v)
  }

  updateTitle()
})

onMounted(() => {
  updateTitle()
})

// Initialize current language
async function init() {
  await loadDayjsLocale(current.value)
}

// Reactive initialization and watch
onMounted(init)
watch(current, init)
</script>

<template>
  <div>
    <ASelect
      v-model:value="current"
      :options="languageOptions"
      size="small"
      style="width: 60px"
    />
  </div>
</template>

<style lang="less" scoped>

</style>
