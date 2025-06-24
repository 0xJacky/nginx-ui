<script setup lang="ts">
import dayjs from 'dayjs'
import loadTranslations from '@/api/translations'
import gettext from '@/gettext'
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

// Language mapping configuration
const localeMap: Record<string, string> = {
  fr: 'fr',
  ja: 'ja',
  ko: 'ko',
  de: 'de',
  zh_CN: 'zh-cn',
  zh_TW: 'zh-tw',
  pt: 'pt',
  es: 'es',
  it: 'it',
  ar: 'ar',
  ru: 'ru',
  tr: 'tr',
  vi: 'vi',
}

// Predefined locale importers for dynamic loading
// This approach works with Vite's static analysis requirements
const localeImporters = {
  'fr': () => import('dayjs/locale/fr'),
  'ja': () => import('dayjs/locale/ja'),
  'ko': () => import('dayjs/locale/ko'),
  'de': () => import('dayjs/locale/de'),
  'zh-cn': () => import('dayjs/locale/zh-cn'),
  'zh-tw': () => import('dayjs/locale/zh-tw'),
  'pt': () => import('dayjs/locale/pt'),
  'es': () => import('dayjs/locale/es'),
  'it': () => import('dayjs/locale/it'),
  'ar': () => import('dayjs/locale/ar'),
  'ru': () => import('dayjs/locale/ru'),
  'tr': () => import('dayjs/locale/tr'),
  'vi': () => import('dayjs/locale/vi'),
}

// Dynamically load dayjs locale files
async function loadDayjsLocale(locale: string) {
  const dayjsLocale = localeMap[locale]

  if (!dayjsLocale) {
    dayjs.locale('en')
    return
  }

  try {
    // Use predefined importer function
    const importer = localeImporters[dayjsLocale]
    if (importer) {
      await importer()
      dayjs.locale(dayjsLocale)
    }
    else {
      // Fallback to English if locale not found
      dayjs.locale('en')
    }
  }
  catch (error) {
    console.warn(`Failed to load dayjs locale: ${dayjsLocale}`, error)
    // Graceful fallback to English
    dayjs.locale('en')
  }
}

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
