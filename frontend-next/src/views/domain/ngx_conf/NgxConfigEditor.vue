<script setup lang="ts">
import CertInfo from '@/views/domain/cert/CertInfo'
// import IssueCert from '@/views/domain/cert/IssueCert'
import DirectiveEditor from '@/views/domain/ngx_conf/directive/DirectiveEditor'
import LocationEditor from '@/views/domain/ngx_conf/LocationEditor'
import {computed, ref} from 'vue'
import {useRoute} from 'vue-router'
import {useGettext} from 'vue3-gettext'

const {$gettext} = useGettext()

const {ngx_config, auto_cert, enabled} = defineProps(['ngx_config', 'auto_cert', 'enabled'])

const route = useRoute()

const current_server_index = ref(0)
const name = ref(route.params.name)

const init_ssl_status = ref(false)

function update_cert_info() {
    // if (name.value && this.$refs['cert-info' + this.current_server_index]) {
    //     this.$refs['cert-info' + this.current_server_index].get()
    // }
}

function change_tls(r: any) {
    if (r) {
        // deep copy servers[0] to servers[1]
        const server = JSON.parse(JSON.stringify(ngx_config.servers[0]))

        ngx_config.servers.push(server)

        current_server_index.value = 1

        const servers = ngx_config.servers

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

        const server_name = directivesMap.value['server_name'][0]

        if (!directivesMap.value['ssl_certificate']) {
            servers[1].directives.splice(server_name.idx + 1, 0, {
                directive: 'ssl_certificate',
                params: ''
            })
        }

        setTimeout(() => {
            if (!directivesMap.value['ssl_certificate_key']) {
                servers[1].directives.splice(server_name.idx + 2, 0, {
                    directive: 'ssl_certificate_key',
                    params: ''
                })
            }
        }, 100)

    } else {
        // remove servers[1]
        current_server_index.value = 0
        if (ngx_config.servers.length === 2) {
            ngx_config.servers.splice(1, 1)
        }
    }
}

const current_server_directives = computed(() => {
    return ngx_config.servers[current_server_index.value].directives
})

const directivesMap = computed(() => {
    const map = <any>{}

    current_server_directives.value.forEach((v: any, k: any) => {
        v.idx = k
        if (map[v.directive]) {
            map[v.directive].push(v)
        } else {
            map[v.directive] = [v]
        }
    })

    return map
})


const support_ssl = computed(() => {
    const servers = ngx_config.servers
    for (const server_key in servers) {
        for (const k in servers[server_key].directives) {
            const v = servers[server_key].directives[k]
            if (v.directive === 'listen' && v.params.indexOf('ssl') > 0) {
                return true
            }
        }
    }
    return false
})


const current_support_ssl = computed(() => {
    if (directivesMap.value.listen) {
        for (const v of directivesMap.value.listen) {
            if (v?.params.indexOf('ssl') > 0) {
                return true
            }
        }
    }
    return false

})

</script>

<template>
    <div>
        <a-form-item :label="$gettext('Enable TLS')" v-if="!support_ssl">
            <a-switch @change="change_tls"/>
        </a-form-item>

        <a-tabs v-model:activeKey="current_server_index">
            <a-tab-pane :tab="'Server '+(k+1)" v-for="(v,k) in ngx_config.servers" :key="k">

                <div class="tab-content">
                    <template v-if="current_support_ssl&&enabled">
                        <cert-info :domain="name" v-if="current_support_ssl"/>
                        <!--                        <issue-cert-->
                        <!--                            :current_server_directives="current_server_directives"-->
                        <!--                            :directives-map="directivesMap"-->
                        <!--                            v-model="auto_cert"-->
                        <!--                        />-->
                    </template>

                    <template v-if="v.comments">
                        <h3 v-translate>Comments</h3>
                        <p style="white-space: pre-wrap;">{{ v.comments }}</p>
                    </template>

                    <directive-editor :ngx_directives="v.directives"/>

                    <location-editor :locations="v.locations"/>
                </div>

            </a-tab-pane>
        </a-tabs>
    </div>

</template>

<style scoped>

</style>
