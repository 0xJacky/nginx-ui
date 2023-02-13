<script setup lang="ts">
import DirectiveEditor from '@/views/domain/ngx_conf/directive/DirectiveEditor.vue'
import LocationEditor from '@/views/domain/ngx_conf/LocationEditor.vue'
import NgxConfigEditor from '@/views/domain/ngx_conf/NgxConfigEditor.vue'
import {useGettext} from 'vue3-gettext'
import domain from '@/api/domain'
import ngx from '@/api/ngx'
import {computed, reactive, ref} from 'vue'
import {message} from 'ant-design-vue'
import {useRouter} from 'vue-router'
import template from '@/api/template'

const {$gettext, interpolate} = useGettext()

const config = reactive({name: ''})
const ngx_config = reactive({
    servers: [{
        directives: [],
        locations: []
    }]
})

const error = reactive({})

const current_step = ref(0)

const enabled = ref(true)

const auto_cert = ref(false)

const update = ref(0)

init()

function init() {
    domain.get_template().then(r => {
        Object.assign(ngx_config, r.tokenized)
    })
}

function save() {
    ngx.build_config(ngx_config).then(r => {
        domain.save(config.name, {name: config.name, content: r.content}).then(() => {
            message.success($gettext('Saved successfully'))

            domain.enable(config.name).then(() => {
                message.success($gettext('Enabled successfully'))
                current_step.value++
                window.scroll({top: 0, left: 0, behavior: 'smooth'})
            }).catch(r => {
                message.error(r.message ?? $gettext('Enable failed'), 5)
            })

        }).catch(r => {
            message.error(interpolate($gettext('Save error %{msg}'), {msg: $gettext(r.message) ?? ''}), 5)
        })
    })
}

const router = useRouter()

function goto_modify() {
    router.push('/domain/' + config.name)
}

function create_another() {
    router.go(0)
}

const has_server_name = computed(() => {
    const servers = ngx_config.servers
    for (const server_key in servers) {
        for (const k in servers[server_key].directives) {
            const v: any = servers[server_key].directives[k]
            if (v.directive === 'server_name' && v.params.trim() !== '') {
                return true
            }
        }
    }

    return false
})
</script>

<template>
    <a-card :title="$gettext('Add Site')">
        <div class="domain-add-container">
            <a-steps :current="current_step" size="small">
                <a-step :title="$gettext('Base information')"/>
                <a-step :title="$gettext('Configure SSL')"/>
                <a-step :title="$gettext('Finished')"/>
            </a-steps>
            <template v-if="current_step===0">
                <a-form layout="vertical">
                    <a-form-item :label="$gettext('Configuration Name')">
                        <a-input v-model:value="config.name"/>
                    </a-form-item>
                </a-form>

                <directive-editor :ngx_directives="ngx_config.servers[0].directives"/>
                <br/>
                <location-editor :locations="ngx_config.servers[0].locations"/>
                <br/>
                <a-alert
                    v-if="!has_server_name"
                    :message="$gettext('Warning')"
                    type="warning"
                    show-icon
                >
                    <template #description>
                        <span v-translate>server_name parameter is required</span>
                    </template>
                </a-alert>
                <br/>
            </template>

            <template v-else-if="current_step===1">

                <ngx-config-editor
                    ref="ngx-config-editor"
                    :ngx_config="ngx_config"
                    v-model:auto_cert="auto_cert"
                    :enabled="enabled"
                />

                <br/>

            </template>

            <a-space v-if="current_step<2">
                <a-button
                    type="primary"
                    @click="save"
                    :disabled="!config.name||!has_server_name"
                >
                    <translate>Next</translate>
                </a-button>
            </a-space>
            <a-result
                v-else-if="current_step===2"
                status="success"
                :title="$gettext('Domain Config Created Successfully')"
            >
                <template #extra>
                    <a-button type="primary" @click="goto_modify">
                        <translate>Modify Config</translate>
                    </a-button>
                    <a-button @click="create_another">
                        <translate>Create Another</translate>
                    </a-button>
                </template>
            </a-result>

        </div>
    </a-card>
</template>

<style lang="less" scoped>
.ant-steps {
    padding: 10px 0 20px 0;
}

.domain-add-container {
    max-width: 800px;
    margin: 0 auto
}
</style>
