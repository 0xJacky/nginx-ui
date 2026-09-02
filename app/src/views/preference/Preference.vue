<script setup lang="ts">
import FooterToolBar from '@/components/FooterToolbar'
import { useGlobalStore } from '@/pinia'
import {
  AccessTokens,
  AppSettings,
  AuthSettings,
  CertSettings,
  ExternalNotify,
  GeoLiteSettings,
  HealthCheckSettings,
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
const globalStore = useGlobalStore()
const isDemoResolved = ref(false)

void systemSettingsStore.getSettings()

const router = useRouter()
const route = useRoute()
const activeKey = ref('server')
const isNginxControlEditing = ref(false)

watch(activeKey, () => {
  router.push({
    query: {
      tab: activeKey.value,
    },
  })
})

onMounted(async () => {
  if (route.query?.tab)
    activeKey.value = route.query.tab.toString()

  await globalStore.ensureDemoFlag()
  if (globalStore.isDemo && activeKey.value === 'terminal')
    activeKey.value = 'server'
  isDemoResolved.value = true
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
          key="health_check"
          :tab="$gettext('Health Check')"
        >
          <HealthCheckSettings />
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
          v-if="isDemoResolved && !globalStore.isDemo"
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
          key="access_tokens"
          :tab="$gettext('Access Tokens')"
        >
          <AccessTokens />
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
          <NginxSettings @control-editing="isNginxControlEditing = $event" />
        </ATabPane>
        <ATabPane
          key="openai"
          :tab="$gettext('LLM')"
        >
          <OpenAISettings />
        </ATabPane>
        <ATabPane
          key="logrotate"
          :tab="$gettext('Logrotate')"
        >
          <LogrotateSettings />
        </ATabPane>
        <ATabPane
          key="geolite"
          :tab="$gettext('GeoLite')"
        >
          <GeoLiteSettings />
        </ATabPane>
      </ATabs>
    </div>
    <FooterToolBar
      v-if="activeKey !== 'external_notify'
        && activeKey !== 'geolite'
        && activeKey !== 'access_tokens'
        && !(activeKey === 'nginx' && isNginxControlEditing)"
    >
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
