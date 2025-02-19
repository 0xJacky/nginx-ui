<script setup lang="ts">
import loadTranslations from '@/api/translations'
import gettext from '@/gettext'
import { useSettingsStore } from '@/pinia'
import dayjs from 'dayjs'

import 'dayjs/locale/fr'
import 'dayjs/locale/ja'
import 'dayjs/locale/ko'
import 'dayjs/locale/de'
import 'dayjs/locale/zh-cn'
import 'dayjs/locale/zh-tw'
import 'dayjs/locale/pt'
import 'dayjs/locale/es'
import 'dayjs/locale/it'
import 'dayjs/locale/ar'
import 'dayjs/locale/ru'
import 'dayjs/locale/tr'
import 'dayjs/locale/vi'

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

function updateTitle() {
  const name = route.meta.name as never as () => string

  document.title = `${name()} | Nginx UI`
}

watch(current, v => {
  loadTranslations(route)
  settings.set_language(v)
  gettext.current = v

  updateTitle()
})

onMounted(() => {
  updateTitle()
})

function init() {
  switch (current.value) {
    case 'fr':
      dayjs.locale('fr')
      break
    case 'ja':
      dayjs.locale('ja')
      break
    case 'ko':
      dayjs.locale('ko')
      break
    case 'de':
      dayjs.locale('de')
      break
    case 'zh_CN':
      dayjs.locale('zh-cn')
      break
    case 'zh_TW':
      dayjs.locale('zh-tw')
      break
    case 'pt':
      dayjs.locale('pt')
      break
    case 'es':
      dayjs.locale('es')
      break
    case 'it':
      dayjs.locale('it')
      break
    case 'ar':
      dayjs.locale('ar')
      break
    case 'ru':
      dayjs.locale('ru')
      break
    case 'tr':
      dayjs.locale('tr')
      break
    case 'vi':
      dayjs.locale('vi')
      break
    default:
      dayjs.locale('en')
  }
}

init()

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
