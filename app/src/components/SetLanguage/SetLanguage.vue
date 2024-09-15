<script setup lang="ts">
import dayjs from 'dayjs'
import { useSettingsStore } from '@/pinia'
import gettext from '@/gettext'
import loadTranslations from '@/api/translations'

import 'dayjs/locale/fr'
import 'dayjs/locale/ja'
import 'dayjs/locale/ko'
import 'dayjs/locale/de'
import 'dayjs/locale/zh-cn'
import 'dayjs/locale/zh-tw'
import 'dayjs/locale/pt'
import 'dayjs/locale/es'
import 'dayjs/locale/it'

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
  loadTranslations(route)
  settings.set_language(v)
  gettext.current = v

  const name = route.meta.name as never as () => string

  document.title = `${name()} | Nginx UI`
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
    case 'en':
      dayjs.locale('en')
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
    default:
      dayjs.locale('zh-cn')
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
