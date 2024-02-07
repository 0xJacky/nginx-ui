<script setup>
import DefaultTheme from 'vitepress/theme'
import {useData, useRoute, useRouter} from 'vitepress'
import {onMounted} from 'vue'
import locales from '../config/locales'

const {Layout} = DefaultTheme

const {lang} = useData()

const route = useRoute()
const router = useRouter()

onMounted(async () => {
  const language = (
    navigator.language
  ).replaceAll('-', '_')

  if (lang.value === 'en'
    && locales[language]
    && !route.path.includes(language)
  ) {
    await router.go(language + '/' + route.path)
  }
})
</script>

<template>
  <Layout/>
</template>

<style scoped lang="less">

</style>
