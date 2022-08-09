<script setup lang="ts">
import {useGettext} from 'vue3-gettext'
import {computed, nextTick, ref, watch} from 'vue'
import {message} from 'ant-design-vue'
import domain from '@/api/domain'
import websocket from '@/lib/websocket'

const {$gettext, interpolate} = useGettext()

const props = defineProps(['directivesMap', 'current_server_directives', 'enabled'])

const emit = defineEmits(['changeEnabled', 'callback', 'update:enabled'])

const issuing_cert = ref(false)

function onchange(r: boolean) {
    emit('changeEnabled', r)
    change_auto_cert(r)
    if (r) {
        job()
    }
}

function job() {
    issuing_cert.value = true

    if (no_server_name.value) {
        message.error($gettext('server_name not found in directives'))
        issuing_cert.value = false
        return
    }

    if (server_name_more_than_one.value) {
        message.error($gettext('server_name parameters more than one'))
        issuing_cert.value = false
        return
    }

    const server_name = props.directivesMap['server_name'][0]

    if (!props.directivesMap['ssl_certificate']) {
        props.current_server_directives.splice(server_name.idx + 1, 0, {
            directive: 'ssl_certificate',
            params: ''
        })
    }

    nextTick(() => {
        if (!props.directivesMap['ssl_certificate_key']) {
            const ssl_certificate = props.directivesMap['ssl_certificate'][0]
            props.current_server_directives.splice(ssl_certificate.idx + 1, 0, {
                directive: 'ssl_certificate_key',
                params: ''
            })
        }
    }).then(() => {
        issue_cert(name.value, callback)
    })
}

function callback(ssl_certificate: string, ssl_certificate_key: string) {
    props.directivesMap['ssl_certificate'][0]['params'] = ssl_certificate
    props.directivesMap['ssl_certificate_key'][0]['params'] = ssl_certificate_key

    emit('callback')
}

function change_auto_cert(r: boolean) {
    if (r) {
        domain.add_auto_cert(name.value).then(() => {
            message.success(interpolate($gettext('Auto-renewal enabled for %{name}'), {name: name.value}))
        }).catch(e => {
            message.error(e.message ?? interpolate($gettext('Enable auto-renewal failed for %{name}'), {name: name.value}))
        })
    } else {
        domain.remove_auto_cert(name.value).then(() => {
            message.success(interpolate($gettext('Auto-renewal disabled for %{name}'), {name: name.value}))
        }).catch(e => {
            message.error(e.message ?? interpolate($gettext('Disable auto-renewal failed for %{name}'), {name: name.value}))
        })
    }
}

const issue_cert = async (server_name: string, callback: Function) => {
    message.info($gettext('Getting the certificate, please wait...'), 15)

    const ws = websocket('/api/cert/issue/' + server_name, false)

    ws.onopen = () => {
        ws.send('go')
    }

    ws.onmessage = m => {
        const r = JSON.parse(m.data)
        switch (r.status) {
            case 'success':
                message.success(r.message, 10)
                break
            case 'info':
                message.info(r.message, 10)
                break
            case 'error':
                message.error(r.message, 10)
                break
        }

        if (r.status === 'success' && r.ssl_certificate !== undefined && r.ssl_certificate_key !== undefined) {
            callback(r.ssl_certificate, r.ssl_certificate_key)
        }

        issuing_cert.value = false
    }
}

const server_name_more_than_one = computed(() => {
    return props.directivesMap['server_name'] && (props.directivesMap['server_name'].length > 1 ||
        props.directivesMap['server_name'][0].params.trim().indexOf(' ') > 0)
})

const no_server_name = computed(() => {
    return props.directivesMap['server_name'].length === 0
})

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

watch(server_name_more_than_one, () => {
    emit('update:enabled', false)
    onchange(false)
})

watch(no_server_name, () => {
    emit('update:enabled', false)
    onchange(false)
})
</script>

<template>
    <div>
        <a-form-item :label="$gettext('Encrypt website with Let\'s Encrypt')">
            <a-switch
                :loading="issuing_cert"
                v-model:checked="enabled"
                @change="onchange"
                :disabled="no_server_name||server_name_more_than_one"
            />
            <a-alert
                v-if="no_server_name||server_name_more_than_one"
                :message="$gettext('Warning')"
                type="warning"
                show-icon
            >
                <template slot="description">
                    <span v-if="no_server_name" v-translate>
                        server_name parameter is required
                    </span>
                    <span v-if="server_name_more_than_one" v-translate>
                        server_name parameters more than one
                    </span>
                </template>
            </a-alert>
        </a-form-item>
        <p v-translate>
            Note: The server_name in the current configuration must be the domain name
            you need to get the certificate.
        </p>
        <p v-translate>
            The certificate for the domain will be checked every hour,
            and will be renewed if it has been more than 1 month since it was last issued.
        </p>
        <p v-translate>
            Make sure you have configured a reverse proxy for .well-known
            directory to HTTPChallengePort (default: 9180) before getting the certificate.
        </p>
    </div>
</template>

<style lang="less" scoped>
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
