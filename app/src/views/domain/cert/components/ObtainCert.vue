<script setup lang="ts">
import { Modal, message } from 'ant-design-vue'
import type { ComputedRef, Ref } from 'vue'
import domain from '@/api/domain'
import AutoCertStepOne from '@/views/domain/cert/components/AutoCertStepOne.vue'
import type { NgxConfig, NgxDirective } from '@/api/ngx'
import type { Props } from '@/views/domain/cert/IssueCert.vue'
import type { DnsChallenge } from '@/api/auto_cert'
import ObtainCertLive from '@/views/domain/cert/components/ObtainCertLive.vue'
import type { CertificateResult } from '@/api/cert'
import type { PrivateKeyType } from '@/constants'

const emit = defineEmits(['update:auto_cert'])

const modalVisible = ref(false)
const step = ref(1)
const directivesMap = inject('directivesMap') as Ref<Record<string, NgxDirective[]>>

const [modal, ContextHolder] = Modal.useModal()

const data = ref({
  dns_credential_id: null,
  challenge_method: 'http01',
  code: '',
  configuration: {
    credentials: {},
    additional: {},
  },
}) as Ref<DnsChallenge>

const modalClosable = ref(true)

provide('data', data)

const save_config = inject('save_config') as () => Promise<void>
const no_server_name = inject('no_server_name') as Ref<boolean>
const props = inject('props') as Props
const issuing_cert = inject('issuing_cert') as Ref<boolean>
const ngx_config = inject('ngx_config') as NgxConfig
const current_server_directives = inject('current_server_directives') as ComputedRef<NgxDirective[]>

const name = computed(() => {
  return directivesMap.value.server_name[0].params.trim()
})

const refObtainCertLive = ref()

const issue_cert = (config_name: string, server_name: string) => {
  refObtainCertLive.value.issue_cert(config_name, server_name.trim().split(' ')).then(resolveCert)
}

async function resolveCert({ ssl_certificate, ssl_certificate_key, key_type }: CertificateResult) {
  directivesMap.value.ssl_certificate[0].params = ssl_certificate
  directivesMap.value.ssl_certificate_key[0].params = ssl_certificate_key
  await save_config()
  change_auto_cert(true, key_type)
  emit('update:auto_cert', true)
}

function change_auto_cert(status: boolean, key_type?: PrivateKeyType) {
  if (status) {
    domain.add_auto_cert(props.configName, {
      domains: name.value.trim().split(' '),
      challenge_method: data.value.challenge_method,
      dns_credential_id: data.value.dns_credential_id,
      key_type: key_type!,
    }).then(() => {
      message.success($gettext('Auto-renewal enabled for %{name}', { name: name.value }))
    }).catch(e => {
      message.error(e.message ?? $gettext('Enable auto-renewal failed for %{name}', { name: name.value }))
    })
  }
  else {
    domain.remove_auto_cert(props.configName).then(() => {
      message.success($gettext('Auto-renewal disabled for %{name}', { name: name.value }))
    }).catch(e => {
      message.error(e.message ?? $gettext('Disable auto-renewal failed for %{name}', { name: name.value }))
    })
  }
}

async function onchange(status: boolean) {
  if (status) {
    job()
  }
  else {
    ngx_config.servers.forEach(v => {
      v.locations = v?.locations?.filter(l => l.path !== '/.well-known/acme-challenge')
    })
    await save_config()
    change_auto_cert(status)
  }

  emit('update:auto_cert', status)
}

function job() {
  modalClosable.value = false
  issuing_cert.value = true

  if (no_server_name.value) {
    message.error($gettext('server_name not found in directives'))
    issuing_cert.value = false

    return
  }

  const server_name_idx = directivesMap.value.server_name[0]?.idx ?? 0

  if (!directivesMap.value.ssl_certificate) {
    current_server_directives.value.splice(server_name_idx + 1, 0, {
      directive: 'ssl_certificate',
      params: '',
    })
  }

  nextTick(() => {
    if (!directivesMap.value.ssl_certificate_key) {
      const ssl_certificate_idx = directivesMap.value.ssl_certificate[0]?.idx ?? 0

      current_server_directives.value.splice(ssl_certificate_idx + 1, 0, {
        directive: 'ssl_certificate_key',
        params: '',
      })
    }
  }).then(() => {
    issue_cert(props.configName, name.value)
  })
}
function toggle(status: boolean) {
  if (status) {
    modal.confirm({
      title: $gettext('Do you want to disable auto-cert renewal?'),
      content: $gettext('We will remove the HTTPChallenge configuration from '
        + 'this file and reload the Nginx. Are you sure you want to continue?'),
      okText: $gettext('OK'),
      cancelText: $gettext('Cancel'),
      mask: false,
      centered: true,
      onOk() {
        onchange(false)
      },
    })
  }
  else {
    modalVisible.value = true
    modalClosable.value = true
  }
}

defineExpose({
  toggle,
})

const can_next = computed(() => {
  if (step.value === 2) {
    return false
  }
  else {
    if (data.value.challenge_method === 'http01')
      return true
    else if (data.value.challenge_method === 'dns01')
      return data.value?.code ?? false
  }
})

function next() {
  step.value++
  onchange(true)
}
</script>

<template>
  <div>
    <ContextHolder />
    <AModal
      v-model:open="modalVisible"
      :title="$gettext('Obtain certificate')"
      :mask-closable="modalClosable"
      :footer="null"
      :closable="modalClosable"
      :width="600"
      force-render
    >
      <template v-if="step === 1">
        <AutoCertStepOne />
      </template>
      <template v-else-if="step === 2">
        <ObtainCertLive
          ref="refObtainCertLive"
          v-model:modal-closable="modalClosable"
          v-model:modal-visible="modalVisible"
        />
      </template>
      <div
        v-if="can_next"
        class="control-btn"
      >
        <AButton
          type="primary"
          @click="next"
        >
          {{ $gettext('Next') }}
        </AButton>
      </div>
    </AModal>
  </div>
</template>

<style lang="less" scoped>
.control-btn {
  display: flex;
  justify-content: flex-end;
}
</style>
