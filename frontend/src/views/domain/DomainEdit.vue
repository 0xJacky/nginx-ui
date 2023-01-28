<script setup lang="ts">
import FooterToolBar from '@/components/FooterToolbar/FooterToolBar.vue'
import CodeEditor from '@/components/CodeEditor/CodeEditor.vue'

import NgxConfigEditor from '@/views/domain/ngx_conf/NgxConfigEditor'
import {useGettext} from 'vue3-gettext'
import {computed, reactive, ref, watch} from 'vue'
import {useRoute, useRouter} from 'vue-router'
import domain from '@/api/domain'
import ngx from '@/api/ngx'
import {message} from 'ant-design-vue'
import config from '@/api/config'


const {$gettext, interpolate} = useGettext()

const route = useRoute()
const router = useRouter()

const name = ref(route.params.name.toString())
watch(route, () => {
    name.value = route.params?.name?.toString() ?? ''
})

const update = ref(0)

const ngx_config = reactive({
    filename: '',
    upstreams: [],
    servers: []
})

const cert_info_map: any = reactive({})

const auto_cert = ref(false)
const enabled = ref(false)
const configText = ref('')
const ok = ref(false)
const advance_mode_ref = ref(false)
const saving = ref(false)
const filename = ref('')
const parse_error_status = ref(false)
const parse_error_message = ref('')

init()

const advance_mode = computed({
    get() {
        return advance_mode_ref.value || parse_error_status.value
    },
    set(v) {
        advance_mode_ref.value = v
    }
})

function handle_response(r: any) {

    Object.keys(cert_info_map).forEach(v => {
        delete cert_info_map[v]
    })
    parse_error_status.value = false
    parse_error_message.value = ''
    filename.value = r.name
    configText.value = r.config
    enabled.value = r.enabled
    auto_cert.value = r.auto_cert
    Object.assign(ngx_config, r.tokenized)
    Object.assign(cert_info_map, r.cert_info)
}

function init() {
    if (name.value) {
        domain.get(name.value).then((r: any) => {
            handle_response(r)
        }).catch(handle_parse_error)
    }
}

function handle_parse_error(r: any) {
    if (r?.error === 'nginx_config_syntax_error') {
        parse_error_status.value = true
        parse_error_message.value = r.message
        config.get('sites-available/' + name.value).then(r => {
            configText.value = r.config
        })
    } else {
        message.error(r.message ?? $gettext('Server error'))
    }

    throw r
}

function on_mode_change(advance_mode: boolean) {
    if (advance_mode) {
        build_config()
    } else {
        return ngx.tokenize_config(configText.value).then((r: any) => {
            Object.assign(ngx_config, r)
        }).catch(handle_parse_error)
    }
}

function build_config() {
    return ngx.build_config(ngx_config).then((r: any) => {
        configText.value = r.content
    })
}

const save = async () => {
    saving.value = true

    if (!advance_mode.value) {
        try {
            await build_config()
        } catch (e) {
            saving.value = false
            message.error($gettext('Failed to save, syntax error(s) was detected in the configuration.'))
            return
        }
    }

    domain.save(name.value, {name: filename.value || name.value, content: configText.value}).then(r => {
        handle_response(r)
        router.push('/domain/' + filename.value)
        message.success($gettext('Saved successfully'))
    }).catch(handle_parse_error).finally(() => {
        saving.value = false
    })

}

function enable() {
    domain.enable(name.value).then(() => {
        message.success($gettext('Enabled successfully'))
        enabled.value = true
    }).catch(r => {
        message.error(interpolate($gettext('Failed to enable %{msg}'), {msg: r.message ?? ''}), 10)
    })
}

function disable() {
    domain.disable(name.value).then(() => {
        message.success($gettext('Disabled successfully'))
        enabled.value = false
    }).catch(r => {
        message.error(interpolate($gettext('Failed to disable %{msg}'), {msg: r.message ?? ''}))
    })
}

function on_change_enabled(checked: boolean) {
    if (checked) {
        enable()
    } else {
        disable()
    }
}
</script>
<template>
    <div>
        <a-card :bordered="false">
            <template #title>
                <span style="margin-right: 10px">{{ interpolate($gettext('Edit %{n}'), {n: name}) }}</span>
                <a-tag color="blue" v-if="enabled">
                    {{ $gettext('Enabled') }}
                </a-tag>
                <a-tag color="orange" v-else>
                    {{ $gettext('Disabled') }}
                </a-tag>
            </template>
            <template #extra>
                <div class="mode-switch">
                    <div class="switch">
                        <a-switch size="small" :disabled="parse_error_status"
                                  v-model:checked="advance_mode" @change="on_mode_change"/>
                    </div>
                    <template v-if="advance_mode">
                        <div>{{ $gettext('Advance Mode') }}</div>
                    </template>
                    <template v-else>
                        <div>{{ $gettext('Basic Mode') }}</div>
                    </template>
                </div>
            </template>

            <transition name="slide-fade">
                <div v-if="advance_mode" key="advance">
                    <div class="parse-error-alert-wrapper" v-if="parse_error_status">
                        <a-alert :message="$gettext('Nginx Configuration Parse Error')"
                                 :description="parse_error_message"
                                 type="error"
                                 show-icon
                        />
                    </div>
                    <div>
                        <code-editor v-model:content="configText"/>
                    </div>
                </div>

                <div class="domain-edit-container" key="basic" v-else>
                    <a-form-item :label="$gettext('Enabled')">
                        <a-switch v-model:checked="enabled" @change="on_change_enabled"/>
                    </a-form-item>
                    <a-form-item :label="$gettext('Name')">
                        <a-input v-model:value="filename"/>
                    </a-form-item>
                    <ngx-config-editor
                        ref="ngx_config_editor"
                        :ngx_config="ngx_config"
                        :cert_info="cert_info_map"
                        v-model:auto_cert="auto_cert"
                        :enabled="enabled"
                        @callback="save()"
                    />
                </div>
            </transition>

        </a-card>

        <footer-tool-bar>
            <a-space>
                <a-button @click="$router.push('/domain/list')">
                    <translate>Back</translate>
                </a-button>
                <a-button type="primary" @click="save" :loading="saving">
                    <translate>Save</translate>
                </a-button>
            </a-space>
        </footer-tool-bar>
    </div>
</template>

<style lang="less">

</style>

<style lang="less" scoped>
.ant-card {
    margin: 10px 0;
    box-shadow: unset;
}

.mode-switch {
    display: flex;

    .switch {
        display: flex;
        align-items: center;
        margin-right: 5px;
    }
}

.parse-error-alert-wrapper {
    margin-bottom: 20px;
}

.domain-edit-container {
    max-width: 800px;
    margin: 0 auto;
}

.slide-fade-enter-active {
    transition: all .5s ease-in-out;
}

.slide-fade-leave-active {
    transition: all .5s cubic-bezier(1.0, 0.5, 0.8, 1.0);
}

.slide-fade-enter, .slide-fade-leave-to
    /* .slide-fade-leave-active for below version 2.1.8 */ {
    transform: translateX(10px);
    opacity: 0;
}

.location-block {

}

.directive-params-wrapper {
    margin: 10px 0;
}

.tab-content {
    padding: 10px;
}
</style>
