<script setup lang="ts">
import FooterToolBar from '@/components/FooterToolbar/FooterToolBar.vue'
import CodeEditor from '@/components/CodeEditor/CodeEditor.vue'

// import NgxConfigEditor from '@/views/domain/ngx_conf/NgxConfigEditor'
import {useGettext} from 'vue3-gettext'
import {reactive, ref} from 'vue'
import {useRoute} from 'vue-router'
import domain from '@/api/domain'
import ngx from '@/api/ngx'
import {message} from 'ant-design-vue'


const {$gettext, interpolate} = useGettext()

const route = useRoute()

const name = ref(route.params.name.toString())
const update = ref(0)
const ngx_config = reactive({
    filename: '',
    upstreams: [],
    servers: []
})

const auto_cert = ref(false)
const enabled = ref(false)
const configText = ref('')
const ok = ref(false)
const advance_mode = ref(false)
const saving = ref(false)

init()

function init() {
    if (name.value) {
        domain.get(name.value).then((r: any) => {
            configText.value = r.config
            enabled.value = r.enabled
            auto_cert.value = r.auto_cert
            Object.assign(ngx_config, r.tokenized)
        }).catch(r => {
            message.error(r.message ?? $gettext('Server error'))
        })
    }
}

function on_mode_change(advance_mode: boolean) {
    if (advance_mode) {
        build_config()
    } else {
        return ngx.tokenize_config(configText.value).then((r: any) => {
            Object.assign(ngx_config, r.tokenized)
        }).catch((e: any) => {
            message.error(e?.message ?? $gettext('Server error'))
        })
    }
}

function build_config() {
    return ngx.build_config(ngx_config).then((r: any) => {
        configText.value = r.content
    }).catch((e: any) => {
        message.error(e?.message ?? $gettext('Server error'))
    })
}

const save = async () => {
    saving.value = true

    if (!advance_mode.value) {
        await build_config()
    }

    domain.save(name.value, {content: configText.value}).then(r => {
        configText.value = r.config
        enabled.value = r.enabled
        Object.assign(ngx_config, r.tokenized)
        message.success($gettext('Saved successfully'))

        // TODO this.$refs.ngx_config.update_cert_info()

    }).catch((e: any) => {
        message.error(e?.message ?? $gettext('Server error'))
    }).finally(() => {
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
</script>
<template>
    <div>
        <a-card :bordered="false">
            <template v-slot:title>
                <span style="margin-right: 10px">{{ interpolate($gettext('Edit %{n}'), {n: name}) }}</span>
                <a-tag color="blue" v-if="enabled">
                    {{ $gettext('Enabled') }}
                </a-tag>
                <a-tag color="orange" v-else>
                    {{ $gettext('Disabled') }}
                </a-tag>
            </template>
            <template v-slot:extra>
                <a-switch size="small" v-model:checked="advance_mode" @change="on_mode_change"/>
                <template v-if="advance_mode">
                    {{ $gettext('Advance Mode') }}
                </template>
                <template v-else>
                    {{ $gettext('Basic Mode') }}
                </template>
            </template>

            <transition name="slide-fade">
                <div v-if="advance_mode" key="advance">
                    <code-editor v-model:content="configText"/>
                </div>

                <div class="domain-edit-container" key="basic" v-else>
                    <a-form-item :label="$gettext('Enabled')">
                        <a-switch v-model="enabled" @change="checked=>{checked?enable():disable()}"/>
                    </a-form-item>

                    <!--                    <ngx-config-editor-->
                    <!--                        ref="ngx_config"-->
                    <!--                        :ngx_config="ngx_config"-->
                    <!--                        v-model="auto_cert"-->
                    <!--                        :enabled="enabled"-->
                    <!--                    />-->
                </div>
            </transition>

        </a-card>

        <footer-tool-bar>
            <a-space>
                <a-button @click="$router.go(-1)">
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

.domain-edit-container {
    max-width: 800px;
    margin: 0 auto;

    /deep/ .ant-form-item-label > label::after {
        content: none;
    }
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
