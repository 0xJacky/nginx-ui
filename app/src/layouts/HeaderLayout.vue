<script setup lang="ts">
import { message } from 'ant-design-vue'
import { HomeOutlined, LogoutOutlined, MenuUnfoldOutlined } from '@ant-design/icons-vue'
import { useRouter } from 'vue-router'
import SetLanguage from '@/components/SetLanguage/SetLanguage.vue'
import gettext from '@/gettext'
import auth from '@/api/auth'
import NginxControl from '@/components/NginxControl/NginxControl.vue'
import SwitchAppearance from '@/components/SwitchAppearance/SwitchAppearance.vue'

const emit = defineEmits<{
  clickUnFold: () => void
}>()

const { $gettext } = gettext

const router = useRouter()

function logout() {
  auth.logout().then(() => {
    message.success($gettext('Logout successful'))
  }).then(() => {
    router.push('/login')
  })
}
</script>

<template>
  <div class="header">
    <div class="tool">
      <MenuUnfoldOutlined @click="emit('clickUnFold')" />
    </div>

    <ASpace
      class="user-wrapper"
      :size="24"
    >
      <SetLanguage class="set_lang" />

      <SwitchAppearance />

      <a href="/">
        <HomeOutlined />
      </a>

      <NginxControl />

      <a @click="logout">
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

.user-wrapper {
  position: absolute;
  right: 28px;
}

.set_lang {
  display: inline;
}
</style>
