<script setup lang="ts">
import {If} from '@/views/domain/ngx_conf'
import CodeEditor from '@/components/CodeEditor'
import {reactive, ref} from 'vue'
import {useGettext} from 'vue3-gettext'
import {CloseOutlined} from '@ant-design/icons-vue'

const {$gettext} = useGettext()

const emit = defineEmits(['save'])

const {ngx_directives, idx} = defineProps(['ngx_directives', 'idx'])

let directive = reactive({directive: '', params: ''})
const adding = ref(false)
const mode = ref('default')


function add() {
    adding.value = true
    directive = reactive({directive: '', params: ''})
}

function save() {
    adding.value = false
    if (mode.value === If) {
        directive.directive = If
    }

    if (idx) {
        ngx_directives.splice(idx + 1, 0, directive)
    } else {
        ngx_directives.push(directive)
    }

    emit('save', idx)
}
</script>

<template>
    <div>
        <div class="add-directive-temp" v-if="adding">
            <a-form-item>
                <a-select v-model:value="mode" default-value="default" style="width: 150px">
                    <a-select-option value="default">
                        {{ $gettext('Single Directive') }}
                    </a-select-option>
                    <a-select-option value="if">
                        if
                    </a-select-option>
                </a-select>
            </a-form-item>
            <a-form-item>
                <code-editor v-if="mode===If" default-height="100px" v-model:content="directive.params"/>

                <a-input-group compact v-else>

                    <a-input style="width: 30%" :placeholder="$gettext('Directive')" v-model="directive.directive"/>

                    <a-input style="width: 70%" :placeholder="$gettext('Params')" v-model="directive.params">
                        <template #suffix>
                            <CloseOutlined @click="adding=false" style="color: rgba(0,0,0,.45);font-size: 10px;"/>
                        </template>
                    </a-input>
                </a-input-group>
            </a-form-item>
        </div>
        <a-button block v-if="!adding" @click="add">{{ $gettext('Add Directive Below') }}</a-button>
        <a-button type="primary" v-else block @click="save"
                  :disabled="!directive.directive&&!directive.params">{{ $gettext('Save Directive') }}
        </a-button>
    </div>
</template>

<style lang="less" scoped>

</style>
