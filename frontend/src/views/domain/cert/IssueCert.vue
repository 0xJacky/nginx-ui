<script setup lang="ts">
import {useGettext} from 'vue3-gettext'
import {computed, inject, nextTick, ref, watch} from 'vue'
import {message, Modal} from 'ant-design-vue'
import domain from '@/api/domain'
import websocket from '@/lib/websocket'
import Template from '@/views/template/Template.vue'
import template from '@/api/template'

const {$gettext, interpolate} = useGettext()

const props = defineProps(['config_name', 'directivesMap', 'current_server_directives',
    'enabled', 'ngx_config'])

const emit = defineEmits(['changeEnabled', 'callback', 'update:enabled'])

const save_site_config: Function = inject('save_site_config')!

const issuing_cert = ref(false)
const modalVisible = ref(false)

const enabled = computed({
    get() {
        return props.enabled
    },
    set(value) {
        emit('update:enabled', value)
    }
})

function confirm() {
    Modal.confirm({
        title: enabled.value ? $gettext('Do you want to disable auto-cert renewal?') :
            $gettext('Do you want to enable auto-cert renewal?'),
        content: enabled.value ? $gettext('We need to add the HTTPChallenge configuration to ' +
                'this file and reload the Nginx. Are you sure you want to continue?') :
            $gettext('We will remove the HTTPChallenge configuration from this file and ' +
                'reload the Nginx configuration file. Are you sure you want to continue?'),
        mask: false,
        centered: true,
        onOk() {
            if (enabled.value) {
                onchange(false)
            } else {
                onchange(true)
            }
        }
    })
}

async function onchange(r: boolean) {
    emit('changeEnabled', r)
    change_auto_cert(r)
    if (r) {
        await template.get_block('letsencrypt.conf').then(r => {
            props.ngx_config.servers.forEach(async (v: any) => {
                v.locations = v.locations.filter((l: any) => l.path !== '/.well-known/acme-challenge')

                v.locations.push(...r.locations)
            })
        })
        // if ssl_certificate is empty, do not save, just use the config from last step.
        if (!props.directivesMap['ssl_certificate']?.[0]) {
            await save_site_config()
        }
        job()
    } else {
        await props.ngx_config.servers.forEach((v: any) => {
            v.locations = v.locations.filter((l: any) => l.path !== '/.well-known/acme-challenge')
        })
        save_site_config()
    }
}

function job() {
    issuing_cert.value = true

    if (no_server_name.value) {
        message.error($gettext('server_name not found in directives'))
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
        issue_cert(props.config_name, name.value, callback)
    })
}

async function callback(ssl_certificate: string, ssl_certificate_key: string) {
    props.directivesMap['ssl_certificate'][0]['params'] = ssl_certificate
    props.directivesMap['ssl_certificate_key'][0]['params'] = ssl_certificate_key
    save_site_config()
}

function change_auto_cert(r: boolean) {
    if (r) {
        domain.add_auto_cert(props.config_name, {domains: name.value.trim().split(' ')}).then(() => {
            message.success(interpolate($gettext('Auto-renewal enabled for %{name}'), {name: name.value}))
        }).catch(e => {
            message.error(e.message ?? interpolate($gettext('Enable auto-renewal failed for %{name}'), {name: name.value}))
        })
    } else {
        domain.remove_auto_cert(props.config_name).then(() => {
            message.success(interpolate($gettext('Auto-renewal disabled for %{name}'), {name: name.value}))
        }).catch(e => {
            message.error(e.message ?? interpolate($gettext('Disable auto-renewal failed for %{name}'), {name: name.value}))
        })
    }
}

const logContainer = ref(null)

function log(msg: string) {
    const para = document.createElement('p')
    para.appendChild(document.createTextNode($gettext(msg)));

    (logContainer.value as any as Node).appendChild(para);

    (logContainer.value as any as Element).scroll({top: 320, left: 0, behavior: 'smooth'})
}

const issue_cert = async (config_name: string, server_name: string, callback: Function) => {
    progressStatus.value = 'active'
    modalClosable.value = false
    modalVisible.value = true
    progressPercent.value = 0;
    (logContainer.value as any as Element).innerHTML = ''

    log($gettext('Getting the certificate, please wait...'))

    const ws = websocket(`/api/domain/${config_name}/cert`, false)

    ws.onopen = () => {
        ws.send(JSON.stringify({
            server_name: server_name.trim().split(' ')
        }))
    }

    ws.onmessage = m => {
        const r = JSON.parse(m.data)
        log(r.message)

        switch (r.status) {
            case 'info':
                progressPercent.value += 5
                break
            default:
                modalClosable.value = true
                issuing_cert.value = false

                if (r.status === 'success' && r.ssl_certificate !== undefined && r.ssl_certificate_key !== undefined) {
                    progressStatus.value = 'success'
                    progressPercent.value = 100
                    callback(r.ssl_certificate, r.ssl_certificate_key)
                } else {
                    progressStatus.value = 'exception'
                }
                break
        }
    }
}

const no_server_name = computed(() => {
    return props.directivesMap['server_name']?.length === 0
})

const name = computed(() => {
    return props.directivesMap['server_name'][0].params.trim()
})

watch(no_server_name, () => {
    emit('update:enabled', false)
    onchange(false)
})

const progressStrokeColor = {
    from: '#108ee9',
    to: '#87d068'
}

const progressPercent = ref(0)

const progressStatus = ref('active')

const modalClosable = ref(false)
</script>

<template>
    <a-modal
        :title="$gettext('Obtaining certificate')"
        v-model:visible="modalVisible"
        :mask-closable="modalClosable"
        :footer="null" :closable="modalClosable" force-render>
        <a-progress
            :stroke-color="progressStrokeColor"
            :percent="progressPercent"
            :status="progressStatus"
        />

        <div class="issue-cert-log-container" ref="logContainer"/>

    </a-modal>
    <div class="issue-cert">
        <a-form-item :label="$gettext('Encrypt website with Let\'s Encrypt')">
            <a-switch
                :loading="issuing_cert"
                :checked="enabled"
                @change="confirm"
                :disabled="no_server_name"
            />
            <a-alert
                v-if="no_server_name"
                :message="$gettext('Warning')"
                type="warning"
                show-icon
            >
                <template slot="description">
                    <span v-if="no_server_name" v-translate>
                        server_name parameter is required
                    </span>
                </template>
            </a-alert>
        </a-form-item>
        <a-alert type="info" closable :message="$gettext('Note')">
            <template #description>
                <p v-translate>
                    The server_name
                    in the current configuration must be the domain name you need to get the certificate, support
                    multiple domains.
                </p>
                <p v-translate>
                    The certificate for the domain will be checked every hour,
                    and will be renewed if it has been more than 1 week since it was last issued.
                </p>
                <p v-translate>
                    Make sure you have configured a reverse proxy for .well-known
                    directory to HTTPChallengePort before obtaining the certificate.
                </p>
            </template>
        </a-alert>
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
