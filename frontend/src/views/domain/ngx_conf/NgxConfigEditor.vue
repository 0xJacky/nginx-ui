<template>
    <a-tabs v-model="current_server_index">
        <a-tab-pane :tab="'Server '+(k+1)" v-for="(v,k) in ngx_config.servers" :key="k">

            <div class="tab-content">
                <template v-if="support_ssl&&enabled">
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
        }
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
        support_ssl: {
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
