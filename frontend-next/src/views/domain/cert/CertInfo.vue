<script setup lang="ts">
import {CloseCircleOutlined, CheckCircleOutlined} from '@ant-design/icons-vue'
import dayjs from 'dayjs'
import {defineProps, reactive, ref} from 'vue'
import domain from '@/api/domain'

const props = defineProps(['domain'])

const ok = ref(false)
const cert = reactive({issuer_name: '', subject_name: '', not_after: '', not_before: ''})

get()

function get() {
    domain.cert_info(props.domain).then((r: any) => {
        Object.assign(cert, r)
        ok.value = true
    }).catch(() => {
        ok.value = false
    })
}
</script>

<template>
    <div class="cert-info" v-if="ok">
        <h2 v-translate>Certificate Status</h2>
        <p v-translate="{issuer: cert.issuer_name}">Intermediate Certification Authorities: %{issuer}</p>
        <p v-translate="{name: cert.subject_name}">Subject Name: %{name}</p>
        <p v-translate="{date: dayjs(cert.not_after).format('YYYY-MM-DD HH:mm:ss').toString()}">
            Expiration Date: %{date}</p>
        <p v-translate="{date: dayjs(cert.not_before).format('YYYY-MM-DD HH:mm:ss').toString()}">
            Not Valid Before: %{date}</p>
        <div class="status">
            <template v-if="new Date().toISOString() < cert.not_before || new Date().toISOString() > cert.not_after">
                <close-circle-outlined style="color: red"/>
                <span v-translate>Certificate has expired</span>
            </template>
            <template v-else>
                <check-circle-outlined style="color: green"/>
                <span v-translate>Certificate is valid</span>
            </template>
        </div>
    </div>
</template>

<style lang="less" scoped>
h4 {
    padding-bottom: 10px;
}

.cert-info {
    padding-bottom: 10px;
}

.status {
    span {
        margin-right: 10px;
    }
}

</style>
