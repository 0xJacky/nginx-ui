<template>
    <div class="cert-info" v-if="ok">
        <h4 v-translate>Certificate Status</h4>
        <p v-translate="{issuer: cert.issuer_name}">Intermediate Certification Authorities: %{issuer}</p>
        <p v-translate="{name: cert.subject_name}">Subject Name: %{name}</p>
        <p v-translate="{date: moment(cert.not_after).format('YYYY-MM-DD HH:mm:ss').toString()}">
            Expiration Date: %{date}</p>
        <p v-translate="{date: moment(cert.not_before).format('YYYY-MM-DD HH:mm:ss').toString()}">
            Not Valid Before: %{date}</p>
        <div class="status">
            <template v-if="new Date().toISOString() < cert.not_before || new Date().toISOString() > cert.not_after">
                <a-icon :style="{ color: 'red' }" type="close-circle"/>
                <span v-translate>Certificate has expired</span>
            </template>
            <template v-else>
                <a-icon :style="{ color: 'green' }" type="check-circle"/>
                <span v-translate>Certificate is valid</span>
            </template>
        </div>
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
h4 {
    padding-bottom: 10px;
}

.cert-info {
    padding-bottom: 10px;
}

.status {
    span {
        margin-left: 10px;
    }
}

</style>
