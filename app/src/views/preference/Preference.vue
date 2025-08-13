<script setup lang="ts">
import FooterToolBar from '@/components/FooterToolbar'
import {
  AppSettings,
  AuthSettings,
  CertSettings,
  ExternalNotify,
  HTTPSettings,
  LogrotateSettings,
  NginxSettings,
  NodeSettings,
  OpenAISettings,
  ServerSettings,
  TerminalSettings,
} from '@/views/preference/tabs'
import useSystemSettingsStore from './store'

const systemSettingsStore = useSystemSettingsStore()

systemSettingsStore.getSettings()

const router = useRouter()
const route = useRoute()
const activeKey = ref('server')

watch(activeKey, () => {
  router.push({
    query: {
      tab: activeKey.value,
    },
  })
})

onMounted(() => {
  if (route.query?.tab)
    activeKey.value = route.query.tab.toString()
})
</script>

<template>
  <ACard :title="$gettext('Preference')">
    <div class="preference-container">
      <ATabs v-model:active-key="activeKey">
        <ATabPane
          key="server"
          :tab="$gettext('Server')"
        >
          <ServerSettings />
        </ATabPane>
        <ATabPane
          key="app"
          :tab="$gettext('App')"
        >
          <AppSettings />
        </ATabPane>
        <ATabPane
          key="external_notify"
          :tab="$gettext('External Notify')"
        >
          <ExternalNotify />
        </ATabPane>
        <ATabPane
          key="node"
          :tab="$gettext('Node')"
        >
          <NodeSettings />
        </ATabPane>
        <ATabPane
          key="http"
          :tab="$gettext('HTTP')"
        >
          <HTTPSettings />
        </ATabPane>
        <ATabPane
          key="terminal"
          :tab="$gettext('Terminal')"
        >
          <TerminalSettings />
        </ATabPane>
        <ATabPane
          key="auth"
          :tab="$gettext('Auth')"
        >
          <AuthSettings />
        </ATabPane>
        <ATabPane
          key="cert"
          :tab="$gettext('Cert')"
        >
          <CertSettings />
        </ATabPane>
        <ATabPane
          key="nginx"
          :tab="$gettext('Nginx')"
        >
          <NginxSettings />
        </ATabPane>
        <ATabPane
          key="openai"
          :tab="$gettext('OpenAI')"
        >
          <OpenAISettings />
        </ATabPane>
        <ATabPane
          key="logrotate"
          :tab="$gettext('Logrotate')"
        >
          <LogrotateSettings />
        </ATabPane>
      </ATabs>
    </div>
    <FooterToolBar v-if="activeKey !== 'external_notify'">
      <AButton
        type="primary"
        @click="systemSettingsStore.save"
      >
        {{ $gettext('Save') }}
      </AButton>
    </FooterToolBar>
  </ACard>
</template>

<style lang="less" scoped>
.preference-container {
  width: 100%;
  max-width: 850px;
  margin: 0 auto;
  padding: 0 10px;

  :deep(label) {
    font-weight: 500;
  }
}
</style>
