<script setup lang="ts">
import type { Cert } from '@/api/cert'
import SelfSignedCertForm from '@/views/certificate/components/SelfSignedCertForm.vue'
import { useTLSDirectives } from '../../composables/useTLSDirectives'
import { useSiteEditorStore } from '../SiteEditor/store'

const editorStore = useSiteEditorStore()
const { curDirectivesMap } = storeToRefs(editorStore)
const { ensureTLSDirectives } = useTLSDirectives()
const { message } = useGlobalApp()

const refForm = useTemplateRef('refForm')

const serverNames = computed(() => {
  const params = curDirectivesMap.value.server_name?.[0]?.params?.trim()
  return params ? params.split(/\s+/) : []
})

function open() {
  refForm.value?.open()
}

async function onCreated(certificate: Cert) {
  ensureTLSDirectives(certificate.ssl_certificate_path, certificate.ssl_certificate_key_path)
  try {
    await editorStore.save()
    message.success($gettext('Self-signed certificate applied'))
  }
  catch {
    message.error($gettext('Certificate written but failed to save site configuration'))
  }
}
</script>

<template>
  <div class="self-signed-cert">
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
