<template>
    <div v-if="ok">
        <h3 v-translate>Certificate Status</h3>
        <p v-translate :translate-params="{issuer: cert.issuer_name}">Intermediate Certification Authorities: %{issuer}</p>
        <p v-translate :translate-params="{name: cert.subject_name}">Subject Name: %{name}</p>
        <p v-translate :translate-params="{date: moment(cert.not_after).format('YYYY-MM-DD HH:mm:ss')}">
            Expiration Date: %{date}</p>
        <p v-translate :translate-params="{date: moment(cert.not_before).format('YYYY-MM-DD HH:mm:ss')}">
            Not Valid Before: %{date}</p>
        <template v-if="new Date().toISOString() < cert.not_before || new Date().toISOString() > cert.not_after">
            <a-icon :style="{ color: 'red' }" type="close-circle"/>
            <translate>Certificate has expired</translate>
        </template>
        <template v-else>
            <a-icon :style="{ color: 'green' }" type="check-circle"/>
            <translate>Certificate is valid</translate>
        </template>
    </div>
</template>

<script>
import moment from 'moment'

export default {
    name: 'CertInfo',
    data() {
        return {
            ok: false,
            cert: {},
            moment
        }
    },
    props: {
        domain: String
    },
    created() {
        this.get()
    },
    watch: {
        domain() {
            this.get()
        }
    },
    methods: {
        get() {
            this.$api.domain.cert_info(this.domain).then(r => {
                this.cert = r
                this.ok = true
            }).catch(() => {
                this.ok = false
            })
        }
    }
}
</script>

<style lang="less" scoped>

</style>
