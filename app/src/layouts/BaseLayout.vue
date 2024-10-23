<script setup lang="ts">
import settings from '@/api/settings'
import PageHeader from '@/components/PageHeader/PageHeader.vue'
import { useSettingsStore } from '@/pinia'
import _ from 'lodash'
import { storeToRefs } from 'pinia'
import FooterLayout from './FooterLayout.vue'
import HeaderLayout from './HeaderLayout.vue'
import SideBar from './SideBar.vue'

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

const { server_name } = storeToRefs(useSettingsStore())

settings.get_server_name().then(r => {
  server_name.value = r.name
})

const breadList = ref([])

provide('breadList', breadList)
</script>

<template>
  <ALayout class="full-screen-wrapper min-h-screen">
    <div class="drawer-sidebar">
      <ADrawer
        v-model:open="drawer_visible"
        :closable="false"
        placement="left"
        width="256"
        @close="drawer_visible = false"
      >
        <SideBar />
      </ADrawer>
    </div>

    <ALayoutSider
      v-model:collapsed="collapsed"
      collapsible
      :style="{ zIndex: 11 }"
      theme="light"
      class="layout-sider"
    >
      <SideBar />
    </ALayoutSider>

    <ALayout class="main-container">
      <ALayoutHeader :style="{ position: 'sticky', top: '0', zIndex: 10, width: '100%' }">
        <HeaderLayout @click-un-fold="drawer_visible = true" />
      </ALayoutHeader>

      <ALayoutContent>
        <PageHeader />
        <div class="router-view">
          <RouterView v-slot="{ Component, route }">
            <Transition name="slide-fade">
              <component
                :is="Component"
                :key="route.path"
              />
            </Transition>
          </RouterView>
        </div>
      </ALayoutContent>

      <ALayoutFooter>
        <FooterLayout />
      </ALayoutFooter>
    </ALayout>
  </ALayout>
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

.ant-layout-header {
  padding: 0 !important;
}

.ant-layout-sider {
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

@media (orientation: landscape) {
  .full-screen-wrapper {
    padding: 0 env(safe-area-inset-right) 0 env(safe-area-inset-left);
  }
}

@media (orientation: portrait) {
  .full-screen-wrapper {
    padding: env(safe-area-inset-top) 0 env(safe-area-inset-bottom);
  }
}
</style>
