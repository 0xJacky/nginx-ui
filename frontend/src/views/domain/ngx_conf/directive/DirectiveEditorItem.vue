<script setup lang="ts">
import CodeEditor from '@/components/CodeEditor'
import {DeleteOutlined, HolderOutlined} from '@ant-design/icons-vue'
import {If} from '@/views/domain/ngx_conf'

import {useGettext} from 'vue3-gettext'
import {onMounted, ref, watch} from 'vue'
import config from '@/api/config'
import {message} from 'ant-design-vue'

const {$gettext, interpolate} = useGettext()

const props = defineProps(['directive', 'current_idx', 'index', 'ngx_directives', 'readonly'])

function remove(index: number) {
    props.ngx_directives.splice(index, 1)
}

const content = ref('')

function init() {
    if (props.directive.directive === 'include')
        config.get(props.directive.params).then(r => {
            content.value = r.config
        })
}

onMounted(init)

watch(props, init)

function save() {
    config.save(props.directive.params, {content: content.value}).then(r => {
        content.value = r.config
        message.success($gettext('Saved successfully'))
    }).catch(r => {
        message.error(interpolate($gettext('Save error %{msg}'), {msg: r.message ?? ''}))
    })
}
</script>

<template>
    <div class="dir-editor-item">
        <div class="input-wrapper">
            <div class="code-editor-wrapper" v-if="directive.directive === ''">
                <HolderOutlined style="padding: 5px"/>
                <code-editor v-model:content="directive.params"
                             defaultHeight="100px" style="width: 100%;"/>
            </div>

            <a-input v-else
                     v-model:value="directive.params" @click="current_idx=index">
                <template #addonBefore>
                    <HolderOutlined/>
                    {{ directive.directive }}
                </template>
            </a-input>

            <a-popconfirm v-if="!readonly"
                          @confirm="remove(index)"
                          :title="$gettext('Are you sure you want to remove this directive?')"
                          :ok-text="$gettext('Yes')"
                          :cancel-text="$gettext('No')">
                <a-button>
                    <template #icon>
                        <DeleteOutlined style="font-size: 14px;"/>
                    </template>
                </a-button>
            </a-popconfirm>
        </div>
        <transition name="slide">
            <div v-if="current_idx===index" class="directive-editor-extra">
                <div class="extra-content">
                    <a-form layout="vertical">
                        <a-form-item :label="$gettext('Comments')">
                            <a-textarea v-model:value="directive.comments"/>
                        </a-form-item>
                        <a-form-item :label="$gettext('Content')" v-if="directive.directive==='include'">
                            <code-editor v-model:content="content"
                                         defaultHeight="200px" style="width: 100%;"/>
                            <div class="save-btn">
                                <a-button @click="save">{{ $gettext('Save') }}</a-button>
                            </div>
                        </a-form-item>
                    </a-form>
                </div>
            </div>
        </transition>
    </div>

</template>

<style lang="less" scoped>
.dir-editor-item {
    margin: 15px 0;
}

.code-editor-wrapper {
    display: flex;
    width: 100%;
    align-items: center;
}

.anticon-holder {
    cursor: grab;
}

.directive-editor-extra {
    background-color: #fafafa;
    padding: 10px 20px;
    margin-bottom: 10px;

    .save-btn {
        display: flex;
        justify-content: flex-end;
        margin-top: 15px;
    }
}

.slide-enter-active, .slide-leave-active {
    transition: max-height .2s ease;
    overflow: hidden;
}

.slide-enter-from, .slide-leave-to {
    max-height: 0;
}

.slide-enter-to, .slide-leave-from {
    max-height: 600px;
}

.input-wrapper {
    display: flex;
    gap: 10px;
    align-items: center;
}
</style>
