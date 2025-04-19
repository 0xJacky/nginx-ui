<script setup lang="ts">
import type { Cert } from '@/api/cert'
import type { Ref } from 'vue'
import cert from '@/api/cert'
import AutoCertForm from '@/components/AutoCertForm/AutoCertForm.vue'
import CertInfo from '@/components/CertInfo/CertInfo.vue'
import CodeEditor from '@/components/CodeEditor/CodeEditor.vue'
import FooterToolBar from '@/components/FooterToolbar/FooterToolBar.vue'
import NodeSelector from '@/components/NodeSelector/NodeSelector.vue'
import { AutoCertState } from '@/constants'
import { message } from 'ant-design-vue'
import RenewCert from './RenewCert.vue'

const route = useRoute()

const id = computed(() => {
  return Number.parseInt(route.params.id as string)
})

const data = ref({}) as Ref<Cert>

const notShowInAutoCert = computed(() => {
  return data.value.auto_cert !== AutoCertState.Enable
})

function init() {
  if (id.value > 0) {
    cert.get(id.value).then(r => {
      data.value = r
    })
  }
  else {
    data.value = {} as Cert
  }
}

onMounted(() => {
  init()
})

const router = useRouter()
const errors = ref({}) as Ref<Record<string, string>>

async function save() {
  try {
    const r = await cert.save(data.value.id, data.value)
    data.value = r
    errors.value = {}
    message.success($gettext('Save successfully'))
    await router.push(`/certificates/${r.id}`)
  }
  // eslint-disable-next-line ts/no-explicit-any
  catch (e: any) {
    errors.value = e.errors
    throw e
  }
}

provide('saveCert', save)

const log = computed(() => {
  const logs = data.value.log?.split('\n')

  logs.forEach((line, idx, lines) => {
    const regex = /\[Nginx UI\] (.*)/

    const matches = line.match(regex)

    if (matches && matches.length > 1) {
      const extractedText = matches[1]

      lines[idx] = line.replaceAll(extractedText, $gettext(extractedText))
    }
  })

  return logs.join('\n')
})

const isManaged = computed(() => {
  return data.value.auto_cert === AutoCertState.Enable
})
</script>

<template>
  <ACard :title="id > 0 ? $gettext('Modify Certificate') : $gettext('Import Certificate')">
    <div
      v-if="isManaged"
      class="mb-4"
    >
      <div class="mb-2">
        <AAlert
          :message="$gettext('This certificate is managed by Nginx UI')"
          type="success"
          show-icon
        />
      </div>
      <div
        v-if="!data.filename"
        class="mt-4 mb-4"
      >
        <AAlert
          :message="$gettext('This Auto Cert item is invalid, please remove it.')"
          type="error"
          show-icon
        />
      </div>
      <div
        v-else-if="!data.domains"
        class="mt-4 mb-4"
      >
        <AAlert
          :message="$gettext('Domains list is empty, try to reopen Auto Cert for %{config}', { config: data.filename })"
          type="error"
          show-icon
        />
      </div>
    </div>

    <ARow>
      <ACol
        :sm="24"
        :md="12"
      >
        <AForm
          v-if="data.certificate_info"
          layout="vertical"
        >
          <AFormItem :label="$gettext('Certificate Status')">
            <CertInfo
              :cert="data.certificate_info"
              class="max-w-96"
            />
          </AFormItem>
        </AForm>

        <template v-if="isManaged">
          <RenewCert
            :options="{
              name: data.name,
              domains: data.domains,
              key_type: data.key_type,
              challenge_method: data.challenge_method,
              dns_credential_id: data.dns_credential_id,
              acme_user_id: data.acme_user_id,
              revoke_old: data.revoke_old,
            }"
            @renewed="init"
          />

          <AutoCertForm
            v-model:options="data"
            style="max-width: 600px"
            hide-note
          />
        </template>

        <AForm
          layout="vertical"
          style="max-width: 600px"
        >
          <AFormItem
            :label="$gettext('Name')"
            :validate-status="errors.name ? 'error' : ''"
            :help="errors.name === 'required'
              ? $gettext('This field is required')
              : ''"
          >
            <p v-if="isManaged">
              {{ data.name }}
            </p>
            <AInput
              v-else
              v-model:value="data.name"
            />
          </AFormItem>
          <AFormItem
            :label="$gettext('SSL Certificate Path')"
            :validate-status="errors.ssl_certificate_path ? 'error' : ''"
            :help="errors.ssl_certificate_path === 'required' ? $gettext('This field is required')
              : errors.ssl_certificate_path === 'certificate_path'
                ? $gettext('The path exists, but the file is not a certificate') : ''"
          >
            <p v-if="isManaged">
              {{ data.ssl_certificate_path }}
            </p>
            <AInput
              v-else
              v-model:value="data.ssl_certificate_path"
            />
          </AFormItem>
          <AFormItem
            :label="$gettext('SSL Certificate Key Path')"
            :validate-status="errors.ssl_certificate_key_path ? 'error' : ''"
            :help="errors.ssl_certificate_key_path === 'required' ? $gettext('This field is required')
              : errors.ssl_certificate_key_path === 'privatekey_path'
                ? $gettext('The path exists, but the file is not a private key') : ''"
          >
            <p v-if="isManaged">
              {{ data.ssl_certificate_key_path }}
            </p>
            <AInput
              v-else
              v-model:value="data.ssl_certificate_key_path"
            />
          </AFormItem>
          <AFormItem :label="$gettext('Sync to')">
            <NodeSelector
              v-model:target="data.sync_node_ids"
              hidden-local
            />
          </AFormItem>
          <AFormItem
            :label="$gettext('SSL Certificate Content')"
            :validate-status="errors.ssl_certificate ? 'error' : ''"
            :help="errors.ssl_certificate === 'certificate'
              ? $gettext('The input is not a SSL Certificate') : ''"
          >
            <CodeEditor
              v-model:content="data.ssl_certificate"
              default-height="300px"
              :readonly="!notShowInAutoCert"
              disable-code-completion
              :placeholder="$gettext('Leave blank will not change anything')"
            />
          </AFormItem>
          <AFormItem
            :label="$gettext('SSL Certificate Key Content')"
            :validate-status="errors.ssl_certificate_key ? 'error' : ''"
            :help="errors.ssl_certificate_key === 'privatekey'
              ? $gettext('The input is not a SSL Certificate Key') : ''"
          >
            <CodeEditor
              v-model:content="data.ssl_certificate_key"
              default-height="300px"
              :readonly="!notShowInAutoCert"
              disable-code-completion
              :placeholder="$gettext('Leave blank will not change anything')"
            />
          </AFormItem>
        </AForm>
      </ACol>
      <ACol
        v-if="data.auto_cert === AutoCertState.Enable"
        :sm="24"
        :md="12"
      >
        <ACard :title="$gettext('Log')">
          <pre
            v-dompurify-html="log"
            class="log-container"
          />
        </ACard>
      </ACol>
    </ARow>

    <FooterToolBar>
      <ASpace>
        <AButton @click="$router.push('/certificates/list')">
          {{ $gettext('Back') }}
        </AButton>

        <AButton
          type="primary"
          @click="save"
        >
          {{ $gettext('Save') }}
        </AButton>
      </ASpace>
    </FooterToolBar>
  </ACard>
</template>

<style scoped lang="less">
.log-container {
  overflow: scroll;
  padding: 5px;
  margin-bottom: 0;

  font-size: 12px;
  line-height: 2;
}
</style>
