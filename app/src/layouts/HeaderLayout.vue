<script setup lang="ts">
import type { ShallowRef } from 'vue'
import auth from '@/api/auth'
import NginxControl from '@/components/NginxControl/NginxControl.vue'
import Notification from '@/components/Notification/Notification.vue'
import SetLanguage from '@/components/SetLanguage/SetLanguage.vue'
import SwitchAppearance from '@/components/SwitchAppearance/SwitchAppearance.vue'
import { DesktopOutlined, HomeOutlined, LogoutOutlined, MenuUnfoldOutlined } from '@ant-design/icons-vue'
import { message } from 'ant-design-vue'
import { useRouter } from 'vue-router'

const emit = defineEmits<{
  clickUnFold: [void]
}>()

const router = useRouter()

function logout() {
  auth.logout().then(() => {
    message.success($gettext('Logout successful'))
  }).then(() => {
    router.push('/login')
  })
}

const headerRef = useTemplateRef('headerRef') as Readonly<ShallowRef<HTMLDivElement>>

const isWorkspace = computed(() => {
  return !!window.inWorkspace
})
</script>

<template>
  <div ref="headerRef" class="header">
    <div class="tool">
      <MenuUnfoldOutlined @click="emit('clickUnFold')" />
    </div>

    <ASpace
      class="user-wrapper"
      :size="24"
    >
      <SetLanguage v-if="!isWorkspace" class="set_lang" />

      <SwitchAppearance />

      <div v-if="!isWorkspace" class="workspace-entry">
        <RouterLink to="/workspace">
          <ATooltip :title="$gettext('Workspace')">
            <DesktopOutlined />
          </ATooltip>
        </RouterLink>
      </div>

      <Notification :header-ref="headerRef" />

      <NginxControl />

      <a href="/">
        <HomeOutlined />
      </a>

      <a v-if="!isWorkspace" @click="logout">
        <LogoutOutlined />
      </a>
    </ASpace>
  </div>
</template>

<style lang="less" scoped>
.header {
  height: 64px;
  padding: 0 20px 0 0;
  background: transparent;
  box-shadow: 0 0 20px 0 rgba(0, 0, 0, 0.05);
  width: 100%;

  a {
    color: #000000;
  }
}

.dark {
  .header {
    box-shadow: 1px 1px 0 0 #404040;

    a {
      color: #fafafa;
    }
  }
}

.tool {
  position: absolute;
  left: 20px;
  @media (min-width: 600px) {
    display: none;
  }
}

.workspace-entry {
  @media (max-width: 600px) {
    display: none;
  }
}

.user-wrapper {
  position: absolute;
  right: 28px;
}

.set_lang {
  display: inline;
}
</style>
