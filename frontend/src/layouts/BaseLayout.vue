<script setup lang="ts">
import HeaderLayout from './HeaderLayout.vue'
import SideBar from './SideBar.vue'
import FooterLayout from './FooterLayout.vue'
import PageHeader from '@/components/PageHeader/PageHeader.vue'
import zh_CN from 'ant-design-vue/es/locale/zh_CN'
import zh_TW from 'ant-design-vue/es/locale/zh_TW'
import en_US from 'ant-design-vue/es/locale/en_US'
import {computed, ref} from 'vue'
import {theme} from 'ant-design-vue'
import _ from 'lodash'

import gettext from '@/gettext'
import {useSettingsStore} from '@/pinia'

const drawer_visible = ref(false)
const collapsed = ref(collapse())

addEventListener('resize', _.throttle(() => {
  collapsed.value = collapse()
}, 50))

function getClientWidth() {
  return document.body.clientWidth
}

function collapse() {
  return getClientWidth() < 1280
}

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
const is_theme_dark = computed(() => settings.theme == 'dark')
</script>

<template>
  <a-config-provider :theme="{
      algorithm: is_theme_dark?theme.darkAlgorithm:theme.defaultAlgorithm,
    }" :locale="lang" :autoInsertSpaceInButton="false">
    <a-layout style="min-height: 100vh">
      <div class="drawer-sidebar">
        <a-drawer
          :closable="false"
          v-model:open="drawer_visible"
          placement="left"
          @close="drawer_visible=false"
          width="256"
        >
          <side-bar/>
        </a-drawer>
      </div>

      <a-layout-sider
        v-model:collapsed="collapsed"
        :collapsible="true"
        :style="{zIndex: 11}"
        theme="light"
        class="layout-sider"
      >
        <side-bar/>
      </a-layout-sider>

      <a-layout>
        <a-layout-header :style="{position: 'sticky', top: '0', zIndex: 10, width:'100%'}">
          <header-layout @clickUnFold="drawer_visible=true"/>
        </a-layout-header>

        <a-layout-content>
          <page-header/>
          <div class="router-view">
            <router-view v-slot="{ Component, route }">
              <transition name="slide-fade">
                <component :is="Component" :key="route.path"/>
              </transition>
            </router-view>
          </div>
        </a-layout-content>

        <a-layout-footer>
          <footer-layout/>
        </a-layout-footer>
      </a-layout>

    </a-layout>
  </a-config-provider>
</template>

<style lang="less" scoped>
.layout-sider {
  @media (max-width: 600px) {
    display: none;
  }
}

.drawer-sidebar {
  @media (min-width: 600px) {
    display: none;
  }
}
</style>

<style lang="less">
.layout-sider .sidebar {
  ul.ant-menu-inline.ant-menu-root {
    height: calc(100vh - 160px);
    overflow-y: auto;
    overflow-x: hidden;

    .ant-menu-item {
      width: unset;
    }
  }

  ul.ant-menu-inline-collapsed {
    height: calc(100vh - 200px);
    overflow-y: auto;
    overflow-x: hidden;
  }
}
</style>

<style lang="less">
.slide-fade-enter-active {
  transition: all .3s ease-in-out;
}

.slide-fade-leave-active {
  transition: all .3s cubic-bezier(1.0, 0.5, 0.8, 1.0);
}

.slide-fade-enter-from, .slide-fade-enter-to, .slide-fade-leave-to
  /* .slide-fade-leave-active for below version 2.1.8 */ {
  transform: translateX(10px);
  opacity: 0;
}

body {
  overflow: unset !important;
}

.dark {
  h1, h2, h3, h4, h5, h6, p {
    color: #fafafa !important;
  }

  .ant-checkbox-indeterminate {
    .ant-checkbox-inner {
      background-color: transparent !important;
    }
  }

  .ant-menu {
    background: unset !important;
  }

  .ant-layout-header {
    background-color: #1f1f1f !important;
  }

  .ant-card {
    background-color: #1f1f1f !important;
  }

  .ant-layout-sider {
    background-color: rgb(20, 20, 20) !important;

    .ant-layout-sider-trigger {
      background-color: rgb(20, 20, 20) !important;
    }

    .ant-menu {
      border-right: 0 !important;
    }

    &.ant-layout-sider-has-trigger {
      padding-bottom: 0;
    }

    box-shadow: 2px 0 8px rgba(29, 35, 41, 0.05);
  }

}

.ant-layout-header {
  padding: 0 !important;
  background-color: #fff !important;
}


.ant-layout-sider {
  background-color: #ffffff;

  &.ant-layout-sider-has-trigger {
    padding-bottom: 0;
  }

  box-shadow: 2px 0 8px rgba(29, 35, 41, 0.05);
}

.ant-drawer-body {
  .sidebar .logo {
    box-shadow: 0 1px 0 0 #e8e8e8;
  }

  .ant-menu-inline, .ant-menu-vertical, .ant-menu-vertical-left {
    border-right: 0 !important;
  }
}


.ant-table-small {
  font-size: 13px;
}

.ant-card-bordered {

}

.header-notice-wrapper .ant-tabs-content {
  max-height: 250px;
}

.header-notice-wrapper .ant-tabs-tabpane-active {
  overflow-y: scroll;
}

.ant-layout-footer {
  @media (max-width: 320px) {
    padding: 10px;
  }
}

.ant-layout-content {
  min-height: auto;

  .router-view {
    padding: 20px;
    @media (max-width: 512px) {
      padding: 20px 0;
    }
    position: relative;
  }
}

.ant-layout-footer {
  text-align: center;
}
</style>
