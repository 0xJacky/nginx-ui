<template>
    <div>
        <a-collapse :bordered="false" default-active-key="1">
            <a-collapse-panel key="1">
                <template v-slot:header>
                    <span style="margin-right: 10px">{{ $gettextInterpolate($gettext('Edit %{n}'), {n: name}) }}</span>
                    <a-tag color="blue" v-if="enabled">
                        {{ $gettext('Enabled') }}
                    </a-tag>
                    <a-tag color="orange" v-else>
                        {{ $gettext('Disabled') }}
                    </a-tag>
                </template>
                <div class="domain-edit-container">
                    <a-form-item :label="$gettext('Enabled')">
                        <a-switch v-model="enabled" @change="checked=>{checked?enable():disable()}"/>
                    </a-form-item>
                    <p v-translate>The following values will only take effect if you have the corresponding fields in your configuration file. The configuration filename cannot be changed after it has been created.</p>
                    <std-data-entry :data-list="columns" v-model="config"/>
                    <template v-if="config.support_ssl">
                        <cert-info :domain="name" ref="cert-info" v-if="name"/>
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
                        <p v-else v-translate>Make sure you have configured a reverse proxy for .well-known directory to HTTPChallengePort (default: 9180) before getting the certificate.</p>
                    </template>
                </div>
            </a-collapse-panel>
        </a-collapse>

        <a-card :title="$gettext('Edit Configuration File')">
            <vue-itextarea v-model="configText"/>
        </a-card>

        <footer-tool-bar>
            <a-space>
                <a-button @click="$router.go(-1)">
                    <translate>Back</translate>
                </a-button>
                <a-button type="primary" @click="save">
                    <translate>Save</translate>
                </a-button>
            </a-space>
        </footer-tool-bar>
    </div>
</template>


<script>
import StdDataEntry from '@/components/StdDataEntry/StdDataEntry'
import FooterToolBar from '@/components/FooterToolbar/FooterToolBar'
import VueItextarea from '@/components/VueItextarea/VueItextarea'
import {columns, columnsSSL} from '@/views/domain/columns'
import {unparse, issue_cert} from '@/views/domain/methods'
import CertInfo from '@/views/domain/CertInfo'
import {$gettext, $interpolate} from '@/lib/translate/gettext'


export default {
    name: 'DomainEdit',
    components: {CertInfo, FooterToolBar, StdDataEntry, VueItextarea},
    data() {
        return {
            name: this.$route.params.name.toString(),
            config: {
                http_listen_port: 80,
                https_listen_port: null,
                server_name: '',
                index: '',
                root: '',
                ssl_certificate: '',
                ssl_certificate_key: '',
                support_ssl: false,
                auto_cert: false
            },
            enabled: false,
            configText: '',
            ws: null,
            ok: false,
            issuing_cert: false
        }
    },
    watch: {
        '$route'() {
            this.init()
        },
        config: {
            handler() {
                this.unparse()
            },
            deep: true
        },
        'config.support_ssl'() {
            if (this.ok) {
                this.change_support_ssl()
            }
        },
        'config.auto_cert'() {
            if (this.ok) {
                this.change_auto_cert()
            }
        }
    },
    created() {
        this.init()
    },
    destroyed() {
        if (this.ws !== null) {
            this.ws.close()
        }
    },
    methods: {
        init() {
            if (this.name) {
                this.$api.domain.get(this.name).then(r => {
                    this.configText = r.config
                    this.config.auto_cert = r.auto_cert
                    this.enabled = r.enabled
                    this.parse(r).then(() => {
                        this.ok = true
                    })
                }).catch(r => {
                    console.log(r)
                    this.$message.error($gettext('Server error'))
                })
            }
        },
        async parse(r) {
            const text = r.config
            const reg = {
                http_listen_port: /listen[\s](.*);/i,
                https_listen_port: /listen[\s](.*) ssl/i,
                server_name: /server_name[\s](.*);/i,
                index: /index[\s](.*);/i,
                root: /root[\s](.*);/i,
                ssl_certificate: /ssl_certificate[\s](.*);/i,
                ssl_certificate_key: /ssl_certificate_key[\s](.*);/i
            }
            this.config['name'] = r.name
            for (let r in reg) {
                const match = text.match(reg[r])
                // console.log(r, match)
                if (match !== null) {
                    if (match[1] !== undefined) {
                        this.config[r] = match[1].trim()
                    } else {
                        this.config[r] = match[0].trim()
                    }
                }
            }
            if (this.config.https_listen_port) {
                this.config.support_ssl = true
            }
        },
        async unparse() {
            this.configText = unparse(this.configText, this.config)
        },
        async get_template() {
            if (this.config.support_ssl) {
                await this.$api.domain.get_template('https-conf').then(r => {
                    this.configText = r.template
                })
            } else {
                await this.$api.domain.get_template('http-conf').then(r => {
                    this.configText = r.template
                })
            }
            await this.unparse()
        },
        change_support_ssl() {
            const that = this
            this.$confirm({
                title: $gettext('Do you want to change the template to support the TLS?'),
                content: $gettext('This operation will lose the custom configuration.'),
                onOk() {
                    that.get_template()
                },
                onCancel() {
                },
            })
        },
        save() {
            this.$api.domain.save(this.name, {content: this.configText}).then(r => {
                this.parse(r)
                this.$message.success($gettext('Saved successfully'))
                if (this.name) {
                    if (this.$refs['cert-info']) this.$refs['cert-info'].get()
                }
            }).catch(r => {
                this.$message.error($interpolate($gettext('Save error %{msg}'), {msg: r.message ?? ''}), 10)
            })
        },
        issue_cert() {
            this.issuing_cert = true
            issue_cert(this.config.server_name, this.callback)
        },
        callback(ssl_certificate, ssl_certificate_key) {
            this.$set(this.config, 'ssl_certificate', ssl_certificate)
            this.$set(this.config, 'ssl_certificate_key', ssl_certificate_key)
            if (this.$refs['cert-info']) this.$refs['cert-info'].get()
            this.issuing_cert = false
        },
        change_auto_cert() {
            if (this.config.auto_cert) {
                this.$api.domain.add_auto_cert(this.name).then(() => {
                    this.$message.success($interpolate($gettext('Auto-renewal enabled for %{name}'), {name: this.name}))
                }).catch(e => {
                    this.$message.error(e.message ?? $interpolate($gettext('Enable auto-renewal failed for %{name}'), {name: this.name}))
                })
            } else {
                this.$api.domain.remove_auto_cert(this.name).then(() => {
                    this.$message.success($interpolate($gettext('Auto-renewal disabled for %{name}'), {name: this.name}))
                }).catch(e => {
                    this.$message.error(e.message ?? $interpolate($gettext('Disable auto-renewal failed for %{name}'), {name: this.name}))
                })
            }
        },
        enable() {
            this.$api.domain.enable(this.name).then(() => {
                this.$message.success($gettext('Enabled successfully'))
                this.enabled = true
            }).catch(r => {
                this.$message.error($interpolate($gettext('Failed to enable %{msg}'), {msg: r.message ?? ''}), 10)
            })
        },
        disable() {
            this.$api.domain.disable(this.name).then(() => {
                this.$message.success($gettext('Disabled successfully'))
                this.enabled = false
            }).catch(r => {
                this.$message.error($interpolate($gettext('Failed to disable %{msg}'), {msg: r.message ?? ''}))
            })
        }
    },
    computed: {
        columns: {
            get() {
                if (this.config.support_ssl) {
                    return [...columns, ...columnsSSL]
                } else {
                    return [...columns]
                }
            }
        },
        is_demo() {
            return this.$store.getters.env.demo === true
        }
    }
}
</script>

<style lang="less">
.ant-collapse {
    background: #ffffff;
    @media (prefers-color-scheme: dark) {
        background: #28292c;
    }
    margin-bottom: 20px;

    .ant-collapse-item {
        border-bottom: unset;
    }
}

.ant-collapse-content-box {
    padding: 24px !important;
}
</style>

<style lang="less" scoped>
.ant-card {
    // margin: 10px;
    @media (max-width: 512px) {
        margin: 10px 0;
    }
}

.domain-edit-container {
    max-width: 800px;
    margin: 0 auto;
    /deep/.ant-form-item-label > label::after {
        content: none;
    }
}

</style>
