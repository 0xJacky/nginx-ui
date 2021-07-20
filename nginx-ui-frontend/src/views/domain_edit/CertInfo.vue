<template>
    <div v-if="ok">
        <h3>证书状态</h3>
        <p>中级证书颁发机构：{{ cert.issuer_name }}</p>
        <p>证书名称：{{ cert.subject_name }}</p>
        <p>过期时间：{{ moment(cert.not_after).format('YYYY-MM-DD HH:mm:ss') }}</p>
        <p>在此之前无效：{{ moment(cert.not_before).format('YYYY-MM-DD HH:mm:ss') }}</p>
        <template v-if="new Date().toISOString() < cert.not_before || new Date().toISOString() > cert.not_after">
            <a-icon :style="{ color: 'red' }" type="close-circle" /> 此证书已过期
        </template>
        <template v-else>
            <a-icon :style="{ color: 'green' }" type="check-circle" /> 证书处在有效期内
        </template>
    </div>
</template>

<script>
import moment from "moment"

export default {
    name: "CertInfo",
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
            }).catch(e => {
                this.$message.error('无法解析 ' + this.domain + ' 的证书信息')
                console.error(e)
                this.ok = false
            })
        }
    }
}
</script>

<style scoped>

</style>
