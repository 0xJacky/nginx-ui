<script setup lang="ts">
import CertInfo from '@/views/domain/cert/CertInfo.vue'
import IssueCert from '@/views/domain/cert/IssueCert.vue'
import {computed, ref} from 'vue'
import {useGettext} from 'vue3-gettext'
import ChangeCert from '@/views/domain/cert/ChangeCert.vue'
import DNSChallenge from '@/views/domain/cert/components/DNSChallenge.vue'

const {$gettext} = useGettext()

const props = defineProps(['config_name', 'directivesMap', 'current_server_directives',
    'enabled', 'ngx_config', 'current_server_index', 'cert_info'])

const emit = defineEmits(['callback', 'update:enabled'])

function callback() {
    emit('callback')
}

const name = computed(() => {
    return props.directivesMap['server_name'][0].params.trim()
})

const enabled = computed({
    get() {
        return props.enabled
    },
    set(value) {
        emit('update:enabled', value)
    }
})

</script>

<template>
    <div>
        <h2 v-translate>Certificate Status</h2>
        <cert-info ref="info" :cert="cert_info"/>

        <change-cert :directives-map="directivesMap"/>

        <issue-cert
            :config_name="config_name"
            :current_server_directives="props.current_server_directives"
            :directives-map="props.directivesMap"
            :ngx_config="ngx_config"
            v-model:enabled="enabled"
            @callback="callback"
        />
    </div>
</template>

<style scoped>

</style>
