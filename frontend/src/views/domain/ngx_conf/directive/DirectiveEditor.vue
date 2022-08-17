<script setup lang="ts">
import CodeEditor from '@/components/CodeEditor'
import {If} from '@/views/domain/ngx_conf'
import DirectiveAdd from '@/views/domain/ngx_conf/directive/DirectiveAdd'
import {useGettext} from 'vue3-gettext'
import {reactive, ref} from 'vue'
import {DeleteOutlined} from '@ant-design/icons-vue'

const {$gettext} = useGettext()

const {ngx_directives} = defineProps<{
    ngx_directives: any[]
}>()

const adding = ref(false)

let directive = reactive({})

const current_idx = ref(-1)

function add() {
    adding.value = true
    directive = reactive({})
}

function save() {
    adding.value = false
    ngx_directives.push(directive)
}

function remove(index: number) {
    ngx_directives.splice(index, 1)
}

function onSave(idx: number) {
    setTimeout(() => {
        current_idx.value = idx + 1
    }, 50)
}
</script>

<template>
    <h2>{{ $gettext('Directives') }}</h2>

    <a-form-item v-for="(directive,index) in ngx_directives" @click="current_idx=index">

        <div class="input-wrapper">
            <code-editor v-if="directive.directive === If" v-model:content="directive.params"
                         defaultHeight="100px" style="width: 100%;"/>

            <a-input v-else
                     :addon-before="directive.directive"
                     v-model:value="directive.params" @click="current_idx=index" @blur="current_idx=-1"/>

            <a-popconfirm @confirm="remove(index)"
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
                    </a-form>
                    <directive-add :ngx_directives="ngx_directives" :idx="index" @save="onSave(index)"/>
                </div>
            </div>
        </transition>
    </a-form-item>

    <directive-add :ngx_directives="ngx_directives"/>
</template>

<style lang="less" scoped>
.directive-editor-extra {
    background-color: #fafafa;
    padding: 10px 20px 20px;
    margin-bottom: 10px;
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
