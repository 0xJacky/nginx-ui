<script setup lang="ts">
import { useGettext } from 'vue3-gettext'
import { message } from 'ant-design-vue'
import type { Ref } from 'vue'
import FooterToolBar from '@/components/FooterToolbar/FooterToolBar.vue'
import settings from '@/api/settings'
import BasicSettings from '@/views/preference/BasicSettings.vue'
import OpenAISettings from '@/views/preference/OpenAISettings.vue'
import NginxSettings from '@/views/preference/NginxSettings.vue'
import type { Settings } from '@/views/preference/typedef'

const { $gettext } = useGettext()

const data = ref<Settings>({
  server: {
    http_host: '0.0.0.0',
    http_port: '9000',
    run_mode: 'debug',
    jwt_secret: '',
    start_cmd: '',
    email: '',
    http_challenge_port: '9180',
    github_proxy: '',
    ca_dir: '',
    node_secret: '',
  },
  nginx: {
    access_log_path: '',
    error_log_path: '',
    config_dir: '',
    pid_path: '',
    reload_cmd: '',
    restart_cmd: '',
  },
  openai: {
    model: '',
    base_url: '',
    proxy: '',
    token: '',
  },
  git: {
    url: '',
    auth_method: '',
    username: '',
    password: '',
    private_key_file_path: '',
  },
})

settings.get().then(r => {
  data.value = r
})

const errors = ref({}) as Ref<Record<string, Record<string, string>>>

async function save() {
  // fix type
  data.value.server.http_challenge_port = data.value.server.http_challenge_port.toString()
  settings.save(data.value).then(r => {
    data.value = r
    message.success($gettext('Save successfully'))
    errors.value = {}
  }).catch(e => {
    errors.value = e.errors
    message.error(e?.message ?? $gettext('Server error'))
  })
}

provide('data', data)
provide('errors', errors)

const router = useRouter()
const route = useRoute()
const activeKey = ref('basic')

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
      <ATabs v-model:activeKey="activeKey">
        <ATabPane
          key="basic"
          :tab="$gettext('Basic')"
        >
          <BasicSettings />
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
      </ATabs>
    </div>
    <FooterToolBar>
      <AButton
        type="primary"
        @click="save"
      >
        {{ $gettext('Save') }}
      </AButton>
    </FooterToolBar>
  </ACard>
</template>

<style lang="less" scoped>
.preference-container {
  width: 100%;
  max-width: 600px;
  margin: 0 auto;
  padding: 0 10px;
}
</style>
