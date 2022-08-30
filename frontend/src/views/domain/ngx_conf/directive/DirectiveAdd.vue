<script setup lang="ts">
import {If} from '@/views/domain/ngx_conf'
import CodeEditor from '@/components/CodeEditor'
import {reactive, ref} from 'vue'
import {useGettext} from 'vue3-gettext'
import {DeleteOutlined} from '@ant-design/icons-vue'

const {$gettext} = useGettext()

const emit = defineEmits(['save'])

const props = defineProps(['ngx_directives', 'idx'])

const directive = reactive({directive: '', params: ''})
const adding = ref(false)
const mode = ref('default')


function add() {
    adding.value = true
    directive.directive = ''
    directive.params = ''
}

function save() {
    adding.value = false
    if (mode.value === If) {
        directive.directive = If
    }

    if (props.idx) {
        props.ngx_directives.splice(props.idx + 1, 0, {directive: directive.directive, params: directive.params})
    } else {
        props.ngx_directives.push({directive: directive.directive, params: directive.params})
    }

    emit('save', props.idx)
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

                <div class="input-wrapper">
                    <code-editor v-if="mode===If" default-height="100px" style="width: 100%;"
                                 v-model:content="directive.params"/>
                    <a-input-group v-else compact>
                        <a-input style="width: 30%" :placeholder="$gettext('Directive')"
                                 v-model:value="directive.directive"/>
                        <a-input style="width: 70%" :placeholder="$gettext('Params')" v-model:value="directive.params"/>
                    </a-input-group>

                    <a-button @click="adding=false">
                        <template #icon>
                            <DeleteOutlined style="font-size: 14px;"/>
                        </template>
                    </a-button>

                </div>
            </a-form-item>
        </div>
        <a-button block v-if="!adding" @click="add">{{ $gettext('Add Directive Below') }}</a-button>
        <a-button type="primary" v-else block @click="save"
                  :disabled="!directive.directive||!directive.params">{{ $gettext('Save Directive') }}
        </a-button>
    </div>
</template>

<style lang="less" scoped>
.input-wrapper {
    display: flex;
    gap: 10px;
    align-items: center;
}
</style>
