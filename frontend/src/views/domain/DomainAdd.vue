<template>
    <a-card :title="$gettext('Add Site')">
        <div class="domain-add-container">
            <a-steps :current="current_step" size="small">
                <a-step :title="$gettext('Base information')" />
                <a-step :title="$gettext('Configure SSL')" />
                <a-step :title="$gettext('Finished')" />
            </a-steps>

            <std-data-entry :data-list="columns" :data-source="config" :error="error" v-show="current_step===0"/>

            <template v-if="current_step===1">
                <a-button
                    @click="issue_cert"
                    type="primary" ghost
                    style="margin: 10px 0"
                    :disabled="is_demo"
                    :loading="issuing_cert"
                >
                    <translate>Getting Certificate from Let's Encrypt</translate>
                </a-button>
                <p v-if="is_demo" v-translate>This feature is not available in demo.</p>

                <std-data-entry :data-list="columnsSSL" :data-source="config" :error="error" />

                <a-space style="margin-right: 10px">
                    <a-button
                        v-if="current_step===1"
                        @click="current_step++"
                    >
                        <translate>Skip</translate>
                    </a-button>
                </a-space>
            </template>

            <a-result
                v-if="current_step===2"
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

            <a-space v-if="current_step<2">
                <a-button
                    type="primary"
                    @click="save"
                    :disabled="!config.name"
                >
                    <translate>Next</translate>
                </a-button>
            </a-space>
        </div>
    </a-card>
</template>

<script>
import StdDataEntry from '@/components/StdDataEntry/StdDataEntry'
import {columns, columnsSSL} from '@/views/domain/columns'
import {unparse, issue_cert} from '@/views/domain/methods'
import $gettext, {$interpolate} from "@/lib/translate/gettext"

export default {
    name: 'DomainAdd',
    components: {StdDataEntry},
    data() {
        return {
            config: {
                http_listen_port: 80,
                https_listen_port: 443
            },
            columns: columns.slice(0, -1), // 隐藏SSL支持开关
            error: {},
            current_step: 0,
            columnsSSL,
            issuing_cert: false
        }
    },
    watch: {
        'config.auto_cert'() {
            this.change_auto_cert()
        }
    },
    methods: {
        save() {
            if (this.current_step===0) {
                this.$api.domain.get_template('http-conf').then(r => {
                    let text = unparse(r.template, this.config)

                    this.$api.domain.save(this.config.name, {content: text, enabled: true}).then(() => {
                        this.$message.success($gettext('Saved successfully'))

                        this.$api.domain.enable(this.config.name).then(() => {
                            this.$message.success($gettext('Enabled successfully'))
                            this.current_step++
                        }).catch(r => {
                            this.$message.error(r.message ?? $gettext('Enable failed'), 10)
                        })

                    }).catch(r => {
                        this.$message.error($interpolate($gettext('Save error %{msg}'), {msg: r.message ?? ""}), 10)
                    })
                })
            } else if (this.current_step === 1) {
                this.$api.domain.get_template('https-conf').then(r => {
                    let text = unparse(r.template, this.config)

                    this.$api.domain.save(this.config.name, {content: text, enabled: true}).then(() => {
                        this.$message.success($gettext('Saved successfully'))
                        this.current_step++
                    }).catch(r => {
                        this.$message.error($interpolate($gettext('Save error %{msg}'), {msg: r.message ?? ""}), 10)
                    })
                })
            }

        },
        issue_cert() {
            this.issuing_cert = true
            issue_cert(this.config.server_name, this.callback)
        },
        callback(ssl_certificate, ssl_certificate_key) {
            this.$set(this.config, 'ssl_certificate', ssl_certificate)
            this.$set(this.config, 'ssl_certificate_key', ssl_certificate_key)
            this.issuing_cert = false
        },
        goto_modify() {
            this.$router.push('/domain/'+this.config.name)
        },
        create_another() {
            this.current_step = 0
            this.config = {
                http_listen_port: 80,
                https_listen_port: 443
            }
        },
        change_auto_cert() {
            if (this.config.auto_cert) {
                this.$api.domain.add_auto_cert(this.config.name).then(() => {
                    this.$message.success($interpolate($gettext('Auto-renewal enabled for %{name}'), {name: this.config.name}))
                }).catch(e => {
                    this.$message.error(e.message ?? $interpolate($gettext('Enable auto-renewal failed for %{name}'), {name: this.config.name}))
                })
            } else {
                this.$api.domain.remove_auto_cert(this.config.name).then(() => {
                    this.$message.success($interpolate($gettext('Auto-renewal disabled for %{name}'), {name: this.config.name}))
                }).catch(e => {
                    this.$message.error(e.message ?? $interpolate($gettext('Disable auto-renewal failed for %{name}'), {name: this.config.name}))
                })
            }
        }
    },
    computed: {
        is_demo() {
            return this.$store.getters.env.demo === true
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
