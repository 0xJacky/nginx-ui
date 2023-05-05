<script setup lang="ts">
import {useGettext} from 'vue3-gettext'
import {provide, ref} from 'vue'
import FooterToolBar from '@/components/FooterToolbar/FooterToolBar.vue'
import {useSettingsStore} from '@/pinia'
import {dark_mode} from '@/lib/theme'
import settings from '@/api/settings'
import {message} from 'ant-design-vue'
import BasicSettings from '@/views/preference/BasicSettings.vue'
import OpenAISettings from '@/views/preference/OpenAISettings.vue'
import NginxLogSettings from '@/views/preference/NginxLogSettings.vue'
import GitSettings from '@/views/preference/GitSettings.vue'
import {IData} from '@/views/preference/typedef'

const {$gettext} = useGettext()

const settingsStore = useSettingsStore()
const theme = ref(settingsStore.theme)
const data = ref<IData>({
    server: {
        http_port: '9000',
        run_mode: 'debug',
        jwt_secret: '',
        start_cmd: '',
        email: '',
        http_challenge_port: '9180',
        github_proxy: ''
    },
    nginx_log: {
        access_log_path: '',
        error_log_path: ''
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
    settingsStore.set_theme(theme.value)
    settingsStore.set_preference_theme(theme.value)
    await dark_mode(theme.value === 'dark')
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
provide('theme', theme)

const activeKey = ref('1')
</script>

<template>
    <a-card :title="$gettext('Preference')">
        <div class="preference-container">
            <a-tabs v-model:activeKey="activeKey">
                <a-tab-pane :tab="$gettext('Basic')" key="1">
                    <basic-settings/>
                </a-tab-pane>
                <a-tab-pane :tab="$gettext('Nginx Log')" key="2">
                    <nginx-log-settings/>
                </a-tab-pane>
                <a-tab-pane :tab="$gettext('OpenAI')" key="3">
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
