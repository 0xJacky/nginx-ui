<template>
    <div>
        <a-collapse :bordered="false" default-active-key="1">
            <a-collapse-panel key="1" :header="name ? $gettextInterpolate($gettext('Edit %{n}'), {n: name}) : $gettext('Add Site')">
                <p v-translate>The following values will only take effect if you have the corresponding fields in your configuration file. The configuration filename cannot be changed after it has been created.</p>
                <std-data-entry :data-list="columns" v-model="config"/>
                <template v-if="config.support_ssl">
                    <cert-info :domain="name" ref="cert-info" v-if="name"/>
                    <br/>
                    <a-button @click="issue_cert" type="primary" ghost v-translate>
                        Getting Certificate from Let's Encrypt
                    </a-button><br/>
                    <p v-translate>Make sure you have configured a reverse proxy for <code>.well-known</code> directory to <code>HTTPChallengePort</code> (default: 9180) before getting the certificate.</p>
                </template>
            </a-collapse-panel>
        </a-collapse>

        <a-card :title="$gettext('Edit Configuration File')">
            <vue-itextarea v-model="configText"/>
        </a-card>

        <footer-tool-bar>
            <a-space>
                <a-button @click="$router.go(-1)"><translate>Cancel</translate></a-button>
                <a-button type="primary" @click="save"><translate>Save</translate></a-button>
            </a-space>
        </footer-tool-bar>
    </div>
</template>


<script>
import StdDataEntry from '@/components/StdDataEntry/StdDataEntry'
import FooterToolBar from '@/components/FooterToolbar/FooterToolBar'
import VueItextarea from '@/components/VueItextarea/VueItextarea'
import {columns, columnsSSL} from '@/views/domain/columns'
import {unparse} from '@/views/domain/methods'
import CertInfo from '@/views/domain/CertInfo'
import $gettext, {$interpolate} from "@/lib/translate/gettext";

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
            configText: '',
            ws: null,
            ok: false
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
                    this.parse(r).then(() => {
                        this.ok = true
                    })
                }).catch(r => {
                    console.log(r)
                    this.$message.error($gettext('Server error'))
                })
            } else {
                this.config = {
                    http_listen_port: 80,
                    https_listen_port: null,
                    server_name: '',
                    index: '',
                    root: '',
                    ssl_certificate: '',
                    ssl_certificate_key: '',
                    support_ssl: false,
                    auto_cert: false,
                }
                this.get_template()
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
                console.log(r)
                this.$message.error($interpolate($gettext('Save error %{msg}'), {msg: r.message ?? ""}), 10)
            })
        },
        issue_cert() {
            this.$message.info($gettext('Note: The server_name in the current configuration must be the domain name you need to get the certificate.'), 15)
            this.$message.info($gettext('Getting the certificate, please wait...'), 15)
            this.ws = new WebSocket(this.getWebSocketRoot() + '/cert/issue/' + this.config.server_name
                + '?token=' + btoa(this.$store.state.user.token))

            this.ws.onopen = () => {
                this.ws.send('go')
            }

            this.ws.onmessage = m => {
                const r = JSON.parse(m.data)
                switch (r.status) {
                    case 'success':
                        this.$message.success(r.message, 10)
                        break
                    case 'info':
                        this.$message.info(r.message, 10)
                        break
                    case 'error':
                        this.$message.error(r.message, 10)
                        break
                }

                if (r.status === 'success' && r.ssl_certificate !== undefined && r.ssl_certificate_key !== undefined) {
                    this.config.ssl_certificate = r.ssl_certificate
                    this.config.ssl_certificate_key = r.ssl_certificate_key
                    if (this.$refs['cert-info']) this.$refs['cert-info'].get()
                }
            }
        },
        change_auto_cert() {
            if (this.config.auto_cert) {
                this.$api.domain.add_auto_cert(this.name).then(() => {
                    this.$message.success($interpolate($gettext('Auto-renewal enabled for %{name}', {name: this.name})))
                }).catch(e => {
                    this.$message.error(e.message ?? $interpolate($gettext('Enable auto-renewal failed for %{name}', {name: this.name})))
                })
            } else {
                this.$api.domain.remove_auto_cert(this.name).then(() => {
                    this.$message.success($interpolate($gettext('Auto-renewal disabled for %{name}', {name: this.name})))
                }).catch(e => {
                    this.$message.error(e.message ?? $interpolate($gettext('Disable auto-renewal failed for %{name}', {name: this.name})))
                })
            }
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

</style>
