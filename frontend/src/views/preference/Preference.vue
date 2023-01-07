<script setup lang="ts">
import {useGettext} from 'vue3-gettext'
import {reactive, ref} from 'vue'
import FooterToolBar from '@/components/FooterToolbar/FooterToolBar.vue'
import {useSettingsStore} from '@/pinia'
import {dark_mode} from '@/lib/theme'
import settings from '@/api/settings'
import {message} from 'ant-design-vue'

const {$gettext} = useGettext()

const settingsStore = useSettingsStore()
const theme = ref(settingsStore.theme)
const data = ref({
    server: {
        http_port: 9000,
        run_mode: 'debug',
        jwt_secret: '',
        start_cmd: '',
        email: '',
        http_challenge_port: 9180
    },
    nginx_log: {
        access_log_path: '',
        error_log_path: ''
    }
})

settings.get().then(r => {
    data.value = r
})

function save() {
    settingsStore.set_theme(theme.value)
    settingsStore.set_preference_theme(theme.value)
    dark_mode(theme.value === 'dark')
    settings.save(data.value).then(r => {
        data.value = r
        message.success($gettext('Save successfully'))
    }).catch(e => {
        message.error(e?.message ?? $gettext('Server error'))
    })
}
</script>

<template>
    <a-card :title="$gettext('Preference')">
        <div class="preference-container">
            <a-form layout="vertical">
                <a-form-item :label="$gettext('HTTP Port')">
                    <p>{{ data.server.http_port }}</p>
                </a-form-item>
                <a-form-item :label="$gettext('Run Mode')">
                    <p>{{ data.server.run_mode }}</p>
                </a-form-item>
                <a-form-item :label="$gettext('Jwt Secret')">
                    <p>{{ data.server.jwt_secret }}</p>
                </a-form-item>
                <a-form-item :label="$gettext('Terminal Start Command')">
                    <p>{{ data.server.start_cmd }}</p>
                </a-form-item>
                <a-form-item :label="$gettext('HTTP Challenge Port')">
                    <a-input-number v-model:value="data.server.http_challenge_port"/>
                </a-form-item>
                <a-form-item :label="$gettext('Theme')">
                    <a-select v-model:value="theme">
                        <a-select-option value="auto">
                            {{ $gettext('Auto') }}
                        </a-select-option>
                        <a-select-option value="light">
                            {{ $gettext('Light') }}
                        </a-select-option>
                        <a-select-option value="dark">
                            {{ $gettext('Dark') }}
                        </a-select-option>
                    </a-select>
                </a-form-item>
                <a-form-item :label="$gettext('Nginx Access Log Path')">
                    <a-input v-model:value="data.nginx_log.access_log_path"/>
                </a-form-item>
                <a-form-item :label="$gettext('Nginx Error Log Path')">
                    <a-input v-model:value="data.nginx_log.error_log_path"/>
                </a-form-item>
            </a-form>
        </div>
    </a-card>
    <footer-tool-bar>
        <a-button type="primary" @click="save">
            {{ $gettext('Save') }}
        </a-button>
    </footer-tool-bar>
</template>

<style lang="less" scoped>
.preference-container {
    width: 100%;
    max-width: 600px;
    margin: 0 auto;
    padding: 0 10px;
}
</style>
