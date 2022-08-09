<script setup lang="ts">
import CertInfo from '@/views/domain/cert/CertInfo.vue'
import IssueCert from '@/views/domain/cert/IssueCert.vue'
import {computed, ref} from 'vue'

const props = defineProps(['directivesMap', 'current_server_directives', 'enabled'])

const emit = defineEmits(['callback', 'update:enabled'])

const info = ref(null)

interface Info {
    get(): void
}

function callback() {
    const t: Info | null = info.value
    t!.get()
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
        <cert-info ref="info" :domain="name" v-if="name"/>
        <issue-cert
            :current_server_directives="props.current_server_directives"
            :directives-map="props.directivesMap"
            v-model:enabled="enabled"
            @callback="callback"
        />
    </div>
</template>

<style scoped>

</style>
