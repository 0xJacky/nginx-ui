<template>
    <div>
        <a-collapse :bordered="false" default-active-key="1">
            <a-collapse-panel key="1" :header="name ? '编辑站点：' + name : '添加站点'">
                <p>您的配置文件中应当有对应的字段时，下列表单中的设置才能生效，配置文件名称创建后不可修改。</p>
                <std-data-entry :data-list="columns" v-model="config"/>
                <template v-if="config.support_ssl">
                    <cert-info :domain="name" ref="cert-info" v-if="name"/>
                    <br/>
                    <a-button @click="issue_cert" type="primary" ghost>
                        自动申请 Let's Encrypt 证书
                    </a-button>
                    <p><br/>点击自动申请证书将会从 Let's Encrypt 获得签发证书
                        在获取签发证书前，请确保配置文件中已为
                        <code>/.well-known</code> 目录反向代理到后端的
                        <code>HTTPChallengePort (default:9180)</code></p>
                </template>
            </a-collapse-panel>
        </a-collapse>

        <a-card title="配置文件编辑">
            <vue-itextarea v-model="configText"/>
        </a-card>

        <footer-tool-bar>
            <a-space>
                <a-button @click="$router.push('/domain/list')">返回</a-button>
                <a-button type="primary" @click="save">保存</a-button>
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
                    this.$message.error('服务器错误')
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
                title: '您已修改 SSL 支持状态，是否需要更换配置文件模板？',
                content: '更换配置文件模板将会丢失自定义配置',
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
                this.$message.success('保存成功')
                if (this.name) {
                    if (this.$refs['cert-info']) this.$refs['cert-info'].get()
                }
            }).catch(r => {
                console.log(r)
                this.$message.error('保存错误' + r.message !== undefined ? ' ' + r.message : null, 10)
            })
        },
        issue_cert() {
            this.$message.info('请注意，当前配置中 server_name 必须为需要申请证书的域名，否则无法申请', 15)
            this.$message.info('正在申请，请稍后', 15)
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
                    this.$message.success(this.name + ' 加入自动续签列表成功')
                }).catch(e => {
                    this.$message.error(e.message ?? this.name + ' 加入自动续签列表失败')
                })
            } else {
                this.$api.domain.remove_auto_cert(this.name).then(() => {
                    this.$message.success('从自动续签列表中删除 ' + this.name + ' 成功')
                }).catch(e => {
                    this.$message.error(e.message ?? '从自动续签列表中删除 ' + this.name + ' 失败')
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
