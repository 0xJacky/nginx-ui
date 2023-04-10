<script setup lang="ts">
import {useGettext} from 'vue3-gettext'
import template from '@/api/template'
import {computed, provide, ref, watch} from 'vue'
import {storeToRefs} from 'pinia'
import {useSettingsStore} from '@/pinia'
import Template from '@/views/template/Template.vue'
import DirectiveEditor from '@/views/domain/ngx_conf/directive/DirectiveEditor.vue'
import LocationEditor from '@/views/domain/ngx_conf/LocationEditor.vue'
import CodeEditor from '@/components/CodeEditor/CodeEditor.vue'
import TemplateForm from '@/views/domain/ngx_conf/config_template/TemplateForm.vue'
import * as wasi from 'wasi'
import _ from 'lodash'

const {$gettext} = useGettext()
const {language} = storeToRefs(useSettingsStore())
const props = defineProps(['ngx_config', 'current_server_index'])

const blocks = ref([])
const data: any = ref({})
const visible = ref(false)
const name = ref('')

function get_block_list() {
    template.get_block_list().then(r => {
        blocks.value = r.data
    })
}

get_block_list()


function view(n: string) {
    visible.value = true
    name.value = n
    template.get_block(n).then(r => {
        data.value = r
    })
}

const trans_description = computed(() => {
    return (item: any) => item.description?.[language.value] ?? item.description?.en ?? ''
})

async function add() {

    if (data.value.custom) {
        props.ngx_config.custom += '\n' + data.value.custom
    }

    props.ngx_config.custom = props.ngx_config.custom.trim()

    if (data.value.locations) {
        props.ngx_config.servers[props.current_server_index].locations.push(...data.value.locations)
    }

    if (data.value.directives) {
        props.ngx_config.servers[props.current_server_index].directives.push(...data.value.directives)
    }

    visible.value = false
}

const variables = computed(() => {
    return data.value.variables
})

function build_template() {
    template.build_block(name.value, variables.value).then(r => {
        data.value.directives = r.directives
        data.value.locations = r.locations
        data.value.custom = r.custom
    })
}

provide('build_template', build_template)
</script>

<template>
    <div>
        <h2 v-translate>Config Templates</h2>
        <div class="config-list-wrapper">
            <a-list
                :grid="{ gutter: 16, xs: 1, sm: 2, md: 2, lg: 2, xl: 2, xxl: 2, xxxl: 2 }"
                :data-source="blocks"
            >
                <template #renderItem="{ item }">
                    <a-list-item>
                        <a-card size="small" :title="item.name">
                            <template #extra>
                                <a-button type="link"
                                          size="small" @click="view(item.filename)">{{ $gettext('View') }}
                                </a-button>
                            </template>
                            <p>{{ $gettext('Author') }}: {{ item.author }}</p>
                            <p>{{ $gettext('Description') }}: {{ trans_description(item) }}</p>
                        </a-card>
                    </a-list-item>
                </template>
            </a-list>
        </div>
        <a-modal
            :title="data.name"
            v-model:visible="visible"
            :mask="false"
            :ok-text="$gettext('Add')"
            @ok="add"
        >
            <p>{{ $gettext('Author') }}: {{ data.author }}</p>
            <p>{{ $gettext('Description') }}: {{ trans_description(data) }}</p>
            <template-form :data="data.variables"/>
            <template v-if="data.custom">
                <h2>{{ $gettext('Custom') }}</h2>
                <code-editor v-model:content="data.custom" default-height="150px"/>
            </template>
            <directive-editor v-if="data.directives" :ngx_directives="data.directives" :readonly="true"/>
            <br/>
            <location-editor v-if="data.locations" :locations="data.locations" :readonly="true"/>
        </a-modal>
    </div>
</template>

<style lang="less" scoped>
.config-list-wrapper {
    max-height: 200px;
    overflow-y: scroll;
    overflow-x: hidden;
}
</style>
