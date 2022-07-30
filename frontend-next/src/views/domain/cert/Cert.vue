<template>
    <div>
        <cert-info ref="info" :domain="name" v-if="name"/>
        <issue-cert
            :current_server_directives="current_server_directives"
            :directives-map="directivesMap"
            v-model="auto_cert"
            @callback="callback"
        />
    </div>
</template>

<script>
import CertInfo from '@/views/domain/cert/CertInfo'
import IssueCert from '@/views/domain/cert/IssueCert'

export default {
    name: 'Cert',
    components: {IssueCert, CertInfo},
    props: {
        directivesMap: Object,
        current_server_directives: Array,
        auto_cert: Boolean
    },
    model: {
        prop: 'auto_cert',
        event: 'change_auto_cert'
    },
    methods: {
        callback() {
            this.$refs.info.get()
        }
    },
    computed: {
        name() {
            return this.directivesMap['server_name'][0].params.trim()
        }
    }
}
</script>

<style scoped>

</style>
