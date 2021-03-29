<template>
    <a-row>
        <a-col :md="12" :sm="24">
            <a-card :title="name ? '编辑站点：' + name : '添加站点'">
                <p>您的配置文件中应当有对应的字段时，下列表单中的设置才能生效。</p>
                <std-data-entry :data-list="columns" v-model="config" @change_support_ssl="change_support_ssl"/>
                <a-button @click="issue_cert" type="primary" v-show="config.support_ssl" ghost>自动申请 Let's Encrypt 证书</a-button>
            </a-card>
        </a-col>
        <a-col :md="12" :sm="24">
            <a-card title="配置文件编辑">
                <vue-itextarea v-model="configText"/>
            </a-card>
        </a-col>
        <footer-tool-bar>
            <a-button type="primary" @click="save">保存</a-button>
        </footer-tool-bar>
    </a-row>
</template>

<script>
import StdDataEntry from "@/components/StdDataEntry/StdDataEntry"
import FooterToolBar from "@/components/FooterToolbar/FooterToolBar"
import VueItextarea from "@/components/VueItextarea/VueItextarea";

const columns = [{
    title: "配置文件名称",
    dataIndex: "name",
    edit: {
        type: "input"
    }
}, {
    title: "网站域名 (server_name)",
    dataIndex: "server_name",
    edit: {
        type: "input"
    }
}, {
    title: "网站根目录 (root)",
    dataIndex: "root",
    edit: {
        type: "input"
    }
}, {
    title: "网站首页 (index)",
    dataIndex: "index",
    edit: {
        type: "input"
    }
}, {
    title: "http 监听端口",
    dataIndex: "http_listen_port",
    edit: {
        type: "number",
        min: 80
    }
}, {
    title: "支持 SSL",
    dataIndex: "support_ssl",
    edit: {
        type: "switch",
        event: "change_support_ssl"
    }
}, {
    title: "https 监听端口",
    dataIndex: "https_listen_port",
    edit: {
        type: "number",
        min: 443
    }
}, {
    title: "SSL 证书路径 (ssl_certificate)",
    dataIndex: "ssl_certificate",
    edit: {
        type: "input"
    }
}, {
    title: "SSL 证书私钥路径 (ssl_certificate_key)",
    dataIndex: "ssl_certificate_key",
    edit: {
        type: "input"
    }
}]

export default {
    name: "DomainEdit",
    components: {FooterToolBar, StdDataEntry, VueItextarea},
    data() {
        return {
            name: this.$route.params.name,
            columns,
            config: {
                http_listen_port: 80,
                https_listen_port: null,
                server_name: "",
                index: "",
                root: "",
                ssl_certificate: "",
                ssl_certificate_key: "",
                support_ssl: false
            },
            configText: "",
            ws: null
        }
    },
    watch: {
        $route() {
            this.config = {}
            this.configText = ""
        },
        config: {
            handler() {
                this.unparse()
            },
            deep: true
        }
    },
    created() {
        if (this.name) {
            this.$api.domain.get(this.name).then(r => {
                this.configText = r.config
                this.parse(r)
            }).catch(r => {
                console.log(r)
                this.$message.error("服务器错误")
            })
        } else {
            this.config = {
                http_listen_port: 80,
                https_listen_port: null,
                server_name: "",
                index: "",
                root: "",
                ssl_certificate: "",
                ssl_certificate_key: "",
                support_ssl: false
            }
            this.get_template()
        }
    },
    destroyed() {
        if (this.ws !== null) {
            this.ws.close()
        }
    },
    methods: {
        parse(r) {
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
                        this.config[r] = match[1]
                    } else {
                        this.config[r] = match[0]
                    }
                }
            }
            if (this.config.https_listen_port) {
                this.config.support_ssl = true
            }
        },
        unparse() {
            let text = this.configText
            // http_listen_port: /listen (.*);/i,
            // https_listen_port: /listen (.*) ssl/i,
            const reg = {
                server_name: /server_name[\s](.*);/ig,
                index: /index[\s](.*);/i,
                root: /root[\s](.*);/i,
                ssl_certificate: /ssl_certificate[\s](.*);/i,
                ssl_certificate_key: /ssl_certificate_key[\s](.*);/i
            }
            text = text.replace(/listen[\s](.*);/i, "listen\t"
                + this.config['http_listen_port'] + ';')
            text = text.replace(/listen[\s](.*) ssl/i, "listen\t"
                + this.config['https_listen_port'] + ' ssl')

            text = text.replace(/listen(.*):(.*);/i, "listen\t[::]:"
                + this.config['http_listen_port'] + ';')
            text = text.replace(/listen(.*):(.*) ssl/i, "listen\t[::]:"
                + this.config['https_listen_port'] + ' ssl')

            for (let k in reg) {
                text = text.replace(new RegExp(reg[k]), k + "\t" +
                    (this.config[k] !== undefined ? this.config[k] : " ") + ";")
            }

            this.configText = text
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
            this.$api.domain.save(this.name ? this.name : this.config.name, {content: this.configText}).then(r => {
                this.parse(r)
                this.$message.success("保存成功")
            }).catch(r => {
                console.log(r)
                this.$message.error("保存错误" + r.message !== undefined ? " " + r.message : null, 10)
            })
        },
        issue_cert() {
            this.$message.info("请注意，当前配置中 server_name 必须为需要申请证书的域名，否则无法申请", 5)
            this.$message.info("正在申请，请稍后")
            this.ws = new WebSocket(this.getWebSocketRoot() + "/cert/issue/" + this.config.server_name
                + "?token=" + process.env["VUE_APP_API_WSS_TOKEN"])

            this.ws.onopen = () => {
                this.ws.send("ping")
            }

            this.ws.onmessage = m => {
                const r = JSON.parse(m.data)
                console.log(r)
                switch (r.status) {
                    case "success":
                        this.$message.success(r.message, 5)
                        break
                    case "info":
                        this.$message.info(r.message, 5)
                        break
                    case "error":
                        this.$message.error(r.message, 5)
                        break
                }

                if (r.status === "success" && r.ssl_certificate !== undefined && r.ssl_certificate_key !== undefined) {
                    this.config.ssl_certificate = r.ssl_certificate
                    this.config.ssl_certificate_key = r.ssl_certificate_key
                }
            }

        }
    }
}
</script>

<style lang="less" scoped>
.ant-card {
    margin: 10px;
    @media (max-width: 512px) {
        margin: 10px 0;
    }
}
</style>
