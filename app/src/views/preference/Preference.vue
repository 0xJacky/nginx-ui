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
      <ATabs
        v-model:active-key="activeKey"
        :items="[
          { key: 'server', label: $gettext('Server') },
          { key: 'app', label: $gettext('App') },
          { key: 'external_notify', label: $gettext('External Notify') },
          { key: 'health_check', label: $gettext('Health Check') },
          { key: 'node', label: $gettext('Node') },
          { key: 'http', label: $gettext('HTTP') },
          ...(isDemoResolved && !globalStore.isDemo
            ? [{ key: 'terminal', label: $gettext('Terminal') }]
            : []),
          { key: 'auth', label: $gettext('Auth') },
          { key: 'access_tokens', label: $gettext('Access Tokens') },
          { key: 'cert', label: $gettext('Cert') },
          { key: 'nginx', label: $gettext('Nginx') },
          { key: 'openai', label: $gettext('LLM') },
          { key: 'logrotate', label: $gettext('Logrotate') },
          { key: 'geolite', label: $gettext('GeoLite') },
        ]"
      >
        <template #contentRender="{ item }">
          <ServerSettings v-if="item.key === 'server'" />
          <AppSettings v-else-if="item.key === 'app'" />
          <ExternalNotify v-else-if="item.key === 'external_notify'" />
          <HealthCheckSettings v-else-if="item.key === 'health_check'" />
          <NodeSettings v-else-if="item.key === 'node'" />
          <HTTPSettings v-else-if="item.key === 'http'" />
          <TerminalSettings v-else-if="item.key === 'terminal'" />
          <AuthSettings v-else-if="item.key === 'auth'" />
          <AccessTokens v-else-if="item.key === 'access_tokens'" />
          <CertSettings v-else-if="item.key === 'cert'" />
          <NginxSettings v-else-if="item.key === 'nginx'" />
          <OpenAISettings v-else-if="item.key === 'openai'" />
          <LogrotateSettings v-else-if="item.key === 'logrotate'" />
          <GeoLiteSettings v-else-if="item.key === 'geolite'" />
        </template>
      </ATabs>
    </div>
    <FooterToolBar v-if="activeKey !== 'external_notify' && activeKey !== 'geolite' && activeKey !== 'access_tokens'">
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
