<template>
    <a-card :title="$gettext('Add Site')">
        <div class="domain-add-container">
            <a-steps :current="current_step" size="small">
                <a-step :title="$gettext('Base information')"/>
                <a-step :title="$gettext('Configure SSL')"/>
                <a-step :title="$gettext('Finished')"/>
            </a-steps>

            <template v-if="current_step===0">
                <a-form-item :label="$gettext('Configuration Name')">
                    <a-input v-model="config.name"/>
                </a-form-item>

                <directive-editor :ngx_directives="ngx_config.servers[0].directives"/>

                <location-editor :locations="ngx_config.servers[0].locations"/>

                <a-alert
                    v-if="!has_server_name"
                    :message="$gettext('Warning')"
                    type="warning"
                    show-icon
                >
                    <template slot="description">
                    <span v-translate>
                        server_name parameter is required
                    </span>
                    </template>
                </a-alert>
                <br/>
            </template>

            <template v-else-if="current_step===1">

                <a-form-item :label="$gettext('Enable TLS')">
                    <a-switch @change="change_tls"/>
                </a-form-item>

                <ngx-config-editor
                    ref="ngx_config"
                    :ngx_config="ngx_config"
                    v-model="auto_cert"
                    :enabled="enabled"
                />

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

<script>
import DirectiveEditor from '@/views/domain/ngx_conf/directive/DirectiveEditor'
import LocationEditor from '@/views/domain/ngx_conf/LocationEditor'
import $gettext, {$interpolate} from '@/lib/translate/gettext'
import NgxConfigEditor from '@/views/domain/ngx_conf/NgxConfigEditor'

export default {
    name: 'DomainAdd',
    components: {NgxConfigEditor, LocationEditor, DirectiveEditor},
    data() {
        return {
            config: {},
            ngx_config: {
                servers: [{}]
            },
            error: {},
            current_step: 0,
            enabled: true,
            auto_cert: false
        }
    },
    created() {
        this.init()
    },
    methods: {
        init() {
            this.$api.domain.get_template().then(r => {
                this.ngx_config = r.tokenized
            })
        },
        save() {
            this.$api.ngx.build_config(this.ngx_config).then(r => {
                this.$api.domain.save(this.config.name, {content: r.content, enabled: true}).then(() => {
                    this.$message.success($gettext('Saved successfully'))

                    this.$api.domain.enable(this.config.name).then(() => {
                        this.$message.success($gettext('Enabled successfully'))
                        this.current_step++
                    }).catch(r => {
                        this.$message.error(r.message ?? $gettext('Enable failed'), 10)
                    })

                }).catch(r => {
                    this.$message.error($interpolate($gettext('Save error %{msg}'), {msg: r.message ?? ''}), 10)
                })
            })
        },
        goto_modify() {
            this.$router.push('/domain/' + this.config.name)
        },
        create_another() {
            this.current_step = 0
            this.config = {}
            this.ngx_config = {
                servers: [{}]
            }
        },
        change_tls(r) {
            if (r) {
                // deep copy servers[0] to servers[1]
                const server = JSON.parse(JSON.stringify(this.ngx_config.servers[0]))

                this.ngx_config.servers.push(server)

                this.$refs.ngx_config.current_server_index = 1

                const servers = this.ngx_config.servers


                let i = 0
                while (i < servers[1].directives.length) {
                    const v = servers[1].directives[i]
                    if (v.directive === 'listen') {
                        servers[1].directives.splice(i, 1)
                    } else {
                        i++
                    }
                }

                servers[1].directives.splice(0, 0, {
                    directive: 'listen',
                    params: '443 ssl http2'
                }, {
                    directive: 'listen',
                    params: '[::]:443 ssl http2'
                })

                const directivesMap = this.$refs.ngx_config.directivesMap

                const server_name = directivesMap['server_name'][0]

                if (!directivesMap['ssl_certificate']) {
                    servers[1].directives.splice(server_name.idx + 1, 0, {
                        directive: 'ssl_certificate',
                        params: ''
                    })
                }

                setTimeout(() => {
                    if (!directivesMap['ssl_certificate_key']) {
                        servers[1].directives.splice(server_name.idx + 2, 0, {
                            directive: 'ssl_certificate_key',
                            params: ''
                        })
                    }
                }, 100)

            } else {
                // remove servers[1]
                this.$refs.ngx_config.current_server_index = 0
                if (this.ngx_config.servers.length === 2) {
                    this.ngx_config.servers.splice(1, 1)
                }
            }
        }
    },
    computed: {
        has_server_name() {
            const servers = this.ngx_config.servers
            for (const server_key in servers) {
                for (const k in servers[server_key].directives) {
                    const v = servers[server_key].directives[k]
                    if (v.directive === 'server_name' && v.params.trim() !== '') {
                        return true
                    }
                }
            }

            return false
        }
    }
}
</script>

<style lang="less" scoped>
.ant-steps {
    padding: 10px 0 20px 0;
}

.domain-add-container {
    max-width: 800px;
    margin: 0 auto
}
</style>
