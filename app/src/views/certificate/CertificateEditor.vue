<script setup lang="ts">
import { useGettext } from 'vue3-gettext'
import type { Ref } from 'vue'
import { message } from 'ant-design-vue'
import { AutoCertState } from '@/constants'
import CertInfo from '@/views/domain/cert/CertInfo.vue'
import AutoCertStepOne from '@/views/domain/cert/components/AutoCertStepOne.vue'
import CodeEditor from '@/components/CodeEditor/CodeEditor.vue'
import type { Cert } from '@/api/cert'
import cert from '@/api/cert'
import FooterToolBar from '@/components/FooterToolbar/FooterToolBar.vue'
import RenewCert from '@/views/certificate/RenewCert.vue'

const { $gettext } = useGettext()

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
function save() {
  cert.save(data.value.id, data.value).then(r => {
    data.value = r
    message.success($gettext('Save successfully'))
    router.push(`/certificates/${r.id}`)
  }).catch(e => {
    message.error($gettext(e?.message ?? 'Server error'))
  })
}

provide('data', data)

provide('no_server_name', computed(() => {
  return false
}))

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
            <CertInfo :cert="data.certificate_info" />
          </AFormItem>
        </AForm>

        <template v-if="isManaged">
          <RenewCert @renewed="init" />

          <AutoCertStepOne
            style="max-width: 600px"
            hide-note
          />
        </template>

        <AForm
          layout="vertical"
          style="max-width: 600px"
        >
          <AFormItem :label="$gettext('Name')">
            <p v-if="isManaged">
              {{ data.name }}
            </p>
            <AInput
              v-else
              v-model:value="data.name"
            />
          </AFormItem>
          <AFormItem :label="$gettext('SSL Certificate Path')">
            <p v-if="isManaged">
              {{ data.ssl_certificate_path }}
            </p>
            <AInput
              v-else
              v-model:value="data.ssl_certificate_path"
            />
          </AFormItem>
          <AFormItem :label="$gettext('SSL Certificate Key Path')">
            <p v-if="isManaged">
              {{ data.ssl_certificate_key_path }}
            </p>
            <AInput
              v-else
              v-model:value="data.ssl_certificate_key_path"
            />
          </AFormItem>
          <AFormItem :label="$gettext('SSL Certificate Content')">
            <CodeEditor
              v-model:content="data.ssl_certificate"
              default-height="300px"
              :readonly="!notShowInAutoCert"
              :placeholder="$gettext('Leave blank will not change anything')"
            />
          </AFormItem>
          <AFormItem :label="$gettext('SSL Certificate Key Content')">
            <CodeEditor
              v-model:content="data.ssl_certificate_key"
              default-height="300px"
              :readonly="!notShowInAutoCert"
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
            class="log-container"
            v-html="log"
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
