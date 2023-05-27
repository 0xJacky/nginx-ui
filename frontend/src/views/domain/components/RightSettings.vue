<script setup lang="ts">
import ChatGPT from '@/components/ChatGPT/ChatGPT.vue'
import {useGettext} from 'vue3-gettext'
import {inject, ref} from 'vue'
import Modal from 'ant-design-vue/lib/modal'
import domain from '@/api/domain'
import {message} from 'ant-design-vue'
import {formatDateTime} from '@/lib/helper'
import Deploy from '@/views/domain/components/Deploy.vue'
import {useSettingsStore} from '@/pinia'

const settings = useSettingsStore()
const {$gettext} = useGettext()
const configText = inject('configText')
const ngx_config = inject('ngx_config')
const enabled = inject('enabled')
const name = inject('name')
const history_chatgpt_record = inject('history_chatgpt_record')
const filename = inject('filename')
const data: any = inject('data')

const active_key = ref(['1', '2', '3'])

function enable() {
    domain.enable(name.value).then(() => {
        message.success($gettext('Enabled successfully'))
        enabled.value = true
    }).catch(r => {
        message.error($gettext('Failed to enable %{msg}', {msg: r.message ?? ''}), 10)
    })
}

function disable() {
    domain.disable(name.value).then(() => {
        message.success($gettext('Disabled successfully'))
        enabled.value = false
    }).catch(r => {
        message.error($gettext('Failed to disable %{msg}', {msg: r.message ?? ''}))
    })
}

function on_change_enabled(checked: boolean) {
    Modal.confirm({
        title: checked ? $gettext('Do you want to enable this site?') : $gettext('Do you want to disable this site?'),
        mask: false,
        centered: true,
        okText: $gettext('OK'),
        cancelText: $gettext('Cancel'),
        async onOk() {
            if (checked) {
                enable()
            } else {
                disable()
            }
        }
    })
}

</script>

<template>
    <a-card class="right-settings">
        <a-collapse v-model:activeKey="active_key" ghost>
            <a-collapse-panel key="1" :header="$gettext('Basic')">
                <a-form-item :label="$gettext('Enabled')">
                    <a-switch :checked="enabled" @change="on_change_enabled"/>
                </a-form-item>
                <a-form-item :label="$gettext('Name')">
                    <a-input v-model:value="filename"/>
                </a-form-item>
                <a-form-item :label="$gettext('Updated at')">
                    {{ formatDateTime(data.modified_at) }}
                </a-form-item>
            </a-collapse-panel>
            <a-collapse-panel key="2" header="Deploy" v-if="!settings.is_remote">
                <deploy/>
            </a-collapse-panel>
            <a-collapse-panel key="3" header="ChatGPT">
                <chat-g-p-t :content="configText" :path="ngx_config.file_name"
                            v-model:history_messages="history_chatgpt_record"/>
            </a-collapse-panel>
        </a-collapse>
    </a-card>
</template>

<style scoped lang="less">
.right-settings {
    position: sticky;
    top: 78px;

    :deep(.ant-card-body) {
        max-height: 100vh;
        overflow-y: scroll;
    }
}

:deep(.ant-collapse-ghost > .ant-collapse-item > .ant-collapse-content > .ant-collapse-content-box) {
    padding: 0;
}

:deep(.ant-collapse > .ant-collapse-item > .ant-collapse-header) {
    padding: 0 0 10px 0;
}
</style>
