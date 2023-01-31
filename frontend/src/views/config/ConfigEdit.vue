<script setup lang="ts">
import FooterToolBar from '@/components/FooterToolbar/FooterToolBar.vue'
import gettext from '@/gettext'
import {useRoute} from 'vue-router'
import {computed, ref} from 'vue'
import config from '@/api/config'
import {message} from 'ant-design-vue'
import CodeEditor from '@/components/CodeEditor/CodeEditor.vue'
import ngx from '@/api/ngx'
import InspectConfig from '@/views/config/InspectConfig.vue'

const {$gettext, interpolate} = gettext
const route = useRoute()

const inspect_config = ref()

const name = computed(() => {
    const n = route.params.name
    if (typeof n === 'string') {
        return n
    }
    return n?.join('/')
})

const configText = ref('')

function init() {
    if (name.value) {
        config.get(name.value).then(r => {
            configText.value = r.config
        }).catch(r => {
            message.error(r.message ?? $gettext('Server error'))
        })
    } else {
        configText.value = ''
    }
}

init()

function save() {
    config.save(name.value, {content: configText.value}).then(r => {
        configText.value = r.config
        message.success($gettext('Saved successfully'))
    }).catch(r => {
        message.error(interpolate($gettext('Save error %{msg}'), {msg: r.message ?? ''}))
    }).finally(() => {
        inspect_config.value.test()
    })
}

function format_code() {
    ngx.format_code(configText.value).then(r => {
        configText.value = r.content
        message.success($gettext('Format successfully'))
    }).catch(r => {
        message.error(interpolate($gettext('Format error %{msg}'), {msg: r.message ?? ''}))
    })
}

</script>


<template>
    <inspect-config ref="inspect_config"/>

    <a-card :title="$gettext('Edit Configuration')">
        <code-editor v-model:content="configText"/>
        <footer-tool-bar>
            <a-space>
                <a-button @click="$router.go(-1)">
                    <translate>Back</translate>
                </a-button>
                <a-button @click="format_code">
                    <translate>Format Code</translate>
                </a-button>
                <a-button type="primary" @click="save">
                    <translate>Save</translate>
                </a-button>
            </a-space>
        </footer-tool-bar>
    </a-card>
</template>

<style lang="less" scoped>

</style>
