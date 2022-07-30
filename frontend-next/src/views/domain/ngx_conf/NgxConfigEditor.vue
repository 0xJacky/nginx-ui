<template>
    <div>
        <a-form-item :label="$gettext('Enable TLS')" v-if="!support_ssl">
            <a-switch @change="change_tls"/>
        </a-form-item>

        <a-tabs v-model="current_server_index">
            <a-tab-pane :tab="'Server '+(k+1)" v-for="(v,k) in ngx_config.servers" :key="k">

                <div class="tab-content">
                    <template v-if="current_support_ssl&&enabled">
                        <cert-info :domain="name" v-if="name"/>
                        <issue-cert
                            :current_server_directives="current_server_directives"
                            :directives-map="directivesMap"
                            v-model="auto_cert"
                        />
                        <cert-info :current_server_directives="current_server_directives"
                                   :directives-map="directivesMap"
                                   v-model="auto_cert"/>
                    </template>

                    <a-form-item :label="$gettext('Comments')" v-if="v.comments">
                        <p style="white-space: pre-wrap;">{{ v.comments }}</p>
                    </a-form-item>

                    <directive-editor :ngx_directives="v.directives" :key="update"/>

                    <location-editor :locations="v.locations"/>
                </div>

            </a-tab-pane>
        </a-tabs>
    </div>

</template>

<script>
import CertInfo from '@/views/domain/cert/CertInfo'
import IssueCert from '@/views/domain/cert/IssueCert'
import DirectiveEditor from '@/views/domain/ngx_conf/directive/DirectiveEditor'
import LocationEditor from '@/views/domain/ngx_conf/LocationEditor'

export default {
    name: 'NgxConfigEditor',
    components: {LocationEditor, DirectiveEditor, IssueCert, CertInfo},
    props: {
        ngx_config: Object,
        auto_cert: Boolean,
        enabled: Boolean
    },
    data() {
        return {
            current_server_index: 0,
            update: 0,
            name: this.$route.params?.name?.toString() ?? '',
            init_ssl_status: false
        }
    },
    model: {
        prop: 'auto_cert',
        event: 'change_auto_cert'
    },
    methods: {
        update_cert_info() {
            if (this.name && this.$refs['cert-info' + this.current_server_index]) {
                this.$refs['cert-info' + this.current_server_index].get()
            }
        },
        change_tls(r) {
            if (r) {
                // deep copy servers[0] to servers[1]
                const server = JSON.parse(JSON.stringify(this.ngx_config.servers[0]))

                this.ngx_config.servers.push(server)

                this.current_server_index = 1

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

                const directivesMap = this.directivesMap

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
                this.current_server_index = 0
                if (this.ngx_config.servers.length === 2) {
                    this.ngx_config.servers.splice(1, 1)
                }
            }
        },
    },
    computed: {
        directivesMap: {
            get() {
                const map = {}

                this.current_server_directives.forEach((v, k) => {
                    v.idx = k
                    if (map[v.directive]) {
                        map[v.directive].push(v)
                    } else {
                        map[v.directive] = [v]
                    }
                })

                return map
            }
        },
        current_server_directives: {
            get() {
                return this.ngx_config.servers[this.current_server_index].directives
            }
        },
        support_ssl() {
            const servers = this.ngx_config.servers
            for (const server_key in servers) {
                for (const k in servers[server_key].directives) {
                    const v = servers[server_key].directives[k]
                    if (v.directive === 'listen' && v.params.indexOf('ssl') > 0) {
                        return true
                    }
                }
            }
            return false
        },
        current_support_ssl: {
            get() {
                if (this.directivesMap.listen) {
                    for (const v of this.directivesMap.listen) {
                        if (v?.params.indexOf('ssl') > 0) {
                            return true
                        }
                    }
                }

                return false
            }
        },
    }
}
</script>

<style scoped>

</style>
