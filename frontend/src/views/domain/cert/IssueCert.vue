<script setup lang="ts">
import {useGettext} from 'vue3-gettext'
import {computed, inject, nextTick, provide, ref, watch} from 'vue'
import Template from '@/views/template/Template.vue'
import ObtainCert from '@/views/domain/cert/components/ObtainCert.vue'

const {$gettext, interpolate} = useGettext()

const props = defineProps(['config_name', 'directivesMap', 'current_server_directives',
    'enabled', 'ngx_config'])

const emit = defineEmits(['callback', 'update:enabled'])

const issuing_cert = ref(false)

const obtain_cert: any = ref()

const enabled = computed({
    get() {
        return props.enabled
    },
    set(value) {
        emit('update:enabled', value)
    }
})

const no_server_name = computed(() => {
    if (props.directivesMap['server_name'] === undefined) {
        return true
    }

    return props.directivesMap['server_name'].length === 0
})

provide('no_server_name', no_server_name)
provide('props', props)
provide('issuing_cert', issuing_cert)

watch(no_server_name, () => emit('update:enabled', false))
const update = ref(0)

async function onchange() {
    update.value++
    await nextTick(() => {
        obtain_cert.value.toggle(enabled.value)
    })
}
</script>

<template>
    <obtain-cert ref="obtain_cert" :key="update"/>
    <div class="issue-cert">
        <a-form-item :label="$gettext('Encrypt website with Let\'s Encrypt')">
            <a-switch
                :loading="issuing_cert"
                :checked="enabled"
                :disabled="no_server_name"
                @change="onchange"
            />
        </a-form-item>
    </div>
</template>

<style lang="less">
.issue-cert-log-container {
    height: 320px;
    overflow: scroll;
    background-color: #f3f3f3;
    border-radius: 4px;
    margin-top: 15px;
    padding: 10px;

    p {
        font-size: 12px;
        line-height: 1.3;
    }
}
</style>

<style lang="less" scoped>
.ant-tag {
    margin: 0;
}

.issue-cert {
    margin: 15px 0;
}

.switch-wrapper {
    position: relative;

    .text {
        position: absolute;
        top: 50%;
        transform: translateY(-50%);
        margin-left: 10px;
    }
}
</style>
