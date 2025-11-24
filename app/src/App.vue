<script setup lang="ts">
import { theme } from 'ant-design-vue'
import en_US from 'ant-design-vue/es/locale/en_US'
import zh_CN from 'ant-design-vue/es/locale/zh_CN'
import zh_TW from 'ant-design-vue/es/locale/zh_TW'
import loadTranslations from '@/api/translations'
import AppProvider from '@/components/AppProvider'
import gettext from '@/gettext'
import { useSettingsStore } from '@/pinia'

const route = useRoute()

const media = window.matchMedia('(prefers-color-scheme: dark)')

function callback() {
  const settings = useSettingsStore()
  if (settings.preference_theme === 'auto') {
    if (media.matches)
      settings.set_theme('dark')
    else
      settings.set_theme('light')
  }
  else {
    settings.set_theme(settings.preference_theme)
  }
}

callback()

const devicePrefersTheme = computed(() => {
  return media.matches ? 'dark' : 'light'
})

provide('devicePrefersTheme', devicePrefersTheme)

media.addEventListener('change', callback)

const lang = computed(() => {
  switch (gettext.current) {
    case 'zh_CN':
      return zh_CN
    case 'zh_TW':
      return zh_TW
    default:
      return en_US
  }
})

const settings = useSettingsStore()
const is_theme_dark = computed(() => settings.theme === 'dark')

loadTranslations(route)
</script>

<template>
  <AConfigProvider
    :theme="{
      algorithm: is_theme_dark ? theme.darkAlgorithm : theme.defaultAlgorithm,
    }"
    :locale="lang"
    :auto-insert-space-in-button="false"
  >
    <AApp>
      <AppProvider>
        <RouterView />
      </AppProvider>
    </AApp>
  </AConfigProvider>
</template>

<style lang="less">
@import "ant-design-vue/dist/reset.css";

.dark {
  h1, h2, h3, h4, h5, h6, p, div {
    color: #fafafa;
  }

  .ant-checkbox-indeterminate {
    .ant-checkbox-inner {
      background-color: transparent !important;
    }
  }

  .ant-layout-header {
    background-color: #141414 !important;
  }

  .ant-layout-sider {
    .ant-menu {
      border-right: 0 !important;
    }

    &.ant-layout-sider-has-trigger {
      padding-bottom: 0;
    }
  }

  // 暗夜模式滚动条样式
  ::-webkit-scrollbar {
    width: 8px;
    height: 8px;
  }

  ::-webkit-scrollbar-track {
    background: #1a1a1a;
    border-radius: 4px;
  }

  ::-webkit-scrollbar-thumb {
    background: #3a3a3a;
    border-radius: 4px;
    transition: background 0.3s ease;

    &:hover {
      background: #4a4a4a;
    }

    &:active {
      background: #555555;
    }
  }

  // Firefox 滚动条样式
  * {
    scrollbar-width: thin;
    scrollbar-color: #3a3a3a #1a1a1a;
  }
}

// 浅色模式滚动条样式
::-webkit-scrollbar {
  width: 8px;
  height: 8px;
}

::-webkit-scrollbar-track {
  background: #f0f0f0;
  border-radius: 4px;
}

::-webkit-scrollbar-thumb {
  background: #d0d0d0;
  border-radius: 4px;
  transition: background 0.3s ease;

  &:hover {
    background: #b0b0b0;
  }

  &:active {
    background: #909090;
  }
}

// Firefox 滚动条样式
* {
  scrollbar-width: thin;
  scrollbar-color: #d0d0d0 #f0f0f0;
}

.ant-layout-header {
  padding: 0 !important;
  background-color: #fff !important;
}

.ant-layout-sider {
  .ant-menu {
    border-inline-end: none !important;
  }
}

@media (max-width: 512px) {
  .ant-card {
    border-radius: 0;
  }
}
</style>

<style lang="less" scoped>

</style>
