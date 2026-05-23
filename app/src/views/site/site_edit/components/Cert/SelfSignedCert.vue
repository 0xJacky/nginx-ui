<script setup lang="ts">
import type { Cert } from '@/api/cert'
import { Modal } from 'ant-design-vue'
import SelfSignedCertForm from '@/views/certificate/components/SelfSignedCertForm.vue'
import { useTLSDirectives } from '../../composables/useTLSDirectives'
import { useSiteEditorStore } from '../SiteEditor/store'

const editorStore = useSiteEditorStore()
const { curDirectivesMap } = storeToRefs(editorStore)
const { ensureTLSDirectives } = useTLSDirectives()
const { message } = useGlobalApp()
const [modal, ContextHolder] = Modal.useModal()

const refForm = useTemplateRef('refForm')

const serverNames = computed(() => {
  const params = curDirectivesMap.value.server_name?.[0]?.params?.trim()
  return params ? params.split(/\s+/) : []
})

function open() {
  refForm.value?.open()
}

function onCreated(certificate: Cert) {
  // Write the TLS directives into the editor first so the user can see the
  // pending diff regardless of whether they choose to save now or review.
  ensureTLSDirectives(certificate.ssl_certificate_path, certificate.ssl_certificate_key_path)

  modal.confirm({
    title: $gettext('Save the site configuration now?'),
    content: $gettext(
      'The certificate has been generated at %{path} and the ssl_certificate '
      + 'directives have been added to the current server block. Save the '
      + 'configuration now, or review the changes in the editor and save manually.',
      { path: certificate.ssl_certificate_path },
    ),
    okText: $gettext('Save now'),
    cancelText: $gettext('Review first'),
    centered: true,
    async onOk() {
      try {
        await editorStore.save()
        message.success($gettext('Self-signed certificate applied'))
      }
      catch {
        message.error($gettext(
          'Saving the site configuration failed; the certificate directives are in '
          + 'the editor — review the changes and retry from the Save button.',
        ))
      }
    },
    onCancel() {
      message.info($gettext('Certificate directives added to the editor; review and save when ready.'))
    },
  })
}
</script>

<template>
  <div class="self-signed-cert">
    <ContextHolder />
    <AFormItem :label="$gettext('Self-signed certificate')">
      <AButton @click="open">
        {{ $gettext('Generate self-signed certificate') }}
      </AButton>
    </AFormItem>
    <SelfSignedCertForm
      ref="refForm"
      :default-domains="serverNames"
      @created="onCreated"
    />
  </div>
</template>

<style scoped lang="less">
.self-signed-cert {
  margin: 15px 0;
}
</style>
