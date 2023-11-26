<script setup lang="ts">
import {useGettext} from 'vue3-gettext'
import {onMounted, provide, ref, watch} from 'vue'
import FooterToolBar from '@/components/FooterToolbar/FooterToolBar.vue'
import settings from '@/api/settings'
import {message} from 'ant-design-vue'
import BasicSettings from '@/views/preference/BasicSettings.vue'
import OpenAISettings from '@/views/preference/OpenAISettings.vue'
import NginxSettings from '@/views/preference/NginxSettings.vue'
import {IData} from '@/views/preference/typedef'
import {useRoute, useRouter} from 'vue-router'

const {$gettext} = useGettext()

const data = ref<IData>({
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
    node_secret: ''
  },
  nginx: {
    access_log_path: '',
    error_log_path: '',
    config_dir: '',
    pid_path: '',
    reload_cmd: '',
    restart_cmd: ''
  },
  openai: {
    model: '',
    base_url: '',
    proxy: '',
    token: ''
  },
  git: {
    url: '',
    auth_method: '',
    username: '',
    password: '',
    private_key_file_path: ''
  }
})

settings.get().then(r => {
  data.value = r
})

async function save() {
  // fix type
  data.value.server.http_challenge_port = data.value.server.http_challenge_port.toString()
  settings.save(data.value).then(r => {
    data.value = r
    message.success($gettext('Save successfully'))
  }).catch(e => {
    message.error(e?.message ?? $gettext('Server error'))
  })
}

provide('data', data)

const router = useRouter()
const route = useRoute()
const activeKey = ref('basic')

watch(activeKey, () => {
  router.push({
    query: {
      tab: activeKey.value
    }
  })
})

onMounted(() => {
  if (route.query?.tab) {
    activeKey.value = route.query.tab.toString()
  }
})
</script>

<template>
  <a-card :title="$gettext('Preference')">
    <div class="preference-container">
      <a-tabs v-model:activeKey="activeKey">
        <a-tab-pane :tab="$gettext('Basic')" key="basic">
          <basic-settings/>
        </a-tab-pane>
        <a-tab-pane :tab="$gettext('Nginx')" key="nginx">
          <nginx-settings/>
        </a-tab-pane>
        <a-tab-pane :tab="$gettext('OpenAI')" key="openai">
          <open-a-i-settings/>
        </a-tab-pane>
      </a-tabs>
    </div>
    <footer-tool-bar>
      <a-button type="primary" @click="save">
        {{ $gettext('Save') }}
      </a-button>
    </footer-tool-bar>
  </a-card>
</template>

<style lang="less" scoped>
.preference-container {
  width: 100%;
  max-width: 600px;
  margin: 0 auto;
  padding: 0 10px;
}
</style>
