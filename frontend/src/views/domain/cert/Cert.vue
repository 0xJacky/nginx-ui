<script setup lang="ts">
import CertInfo from '@/views/domain/cert/CertInfo.vue'
import IssueCert from '@/views/domain/cert/IssueCert.vue'
import {computed, ref} from 'vue'

const {directivesMap, current_server_directives, enabled} = defineProps<{
    directivesMap: any
    current_server_directives: Array<any>
    enabled: boolean
}>()

const info = ref(null)

interface Info {
    get(): void
}

function callback() {
    const t: Info | null = info.value
    t!.get()
}

const name = computed(() => {
    return directivesMap['server_name'][0].params.trim()
})
</script>

<template>
    <div>
        <cert-info ref="info" :domain="name" v-if="name"/>
        <issue-cert
            :current_server_directives="current_server_directives"
            :directives-map="directivesMap"
            v-model:enabled="enabled"
            @callback="callback"
        />
    </div>
</template>

<style scoped>

</style>
