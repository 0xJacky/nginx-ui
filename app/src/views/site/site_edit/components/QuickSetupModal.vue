<script setup lang="ts">
import template from '@/api/template'
import { useNgxConfigStore } from '@/components/NgxConfigEditor'
import QuickSetupForm from '@/views/site/components/QuickSetup/QuickSetupForm.vue'
import { useQuickConfig } from '@/views/site/components/QuickSetup/useQuickConfig'
import { useSiteEditorStore } from './SiteEditor/store'

const open = defineModel<boolean>('open', { default: false })

const { message, modal } = useGlobalApp()
const route = useRoute()

const siteName = computed(() => decodeURIComponent(route.params?.name?.toString() ?? ''))

const editorStore = useSiteEditorStore()
const { advanceMode, configText } = storeToRefs(editorStore)

const ngxConfigStore = useNgxConfigStore()
const { ngxConfig } = storeToRefs(ngxConfigStore)

const quick = useQuickConfig()
const { quickFormValid, quickGenerating } = quick
const quickAnalyzing = ref(false)

watch(open, async isOpen => {
  if (!isOpen)
    return

  quick.reset()
  quickAnalyzing.value = true
  try {
    const content = advanceMode.value
      ? configText.value
      : await editorStore.buildConfig()
    const r = await template.analyze_quick_config(content)
    quick.applyInitial(r.request)
  }
  catch {
    // Fall back to default form values when the existing config cannot be analyzed.
  }
  finally {
    // The name is derived from the site itself and is not editable here.
    quick.state.name = siteName.value
    quickAnalyzing.value = false
  }
})

async function generateConfig() {
  const r = await quick.generate()

  modal.confirm({
    title: $gettext('Replace configuration?'),
    content: $gettext('The generated configuration will replace the current one. Any custom directives or locations will be lost.'),
    okText: $gettext('Replace'),
    cancelText: $gettext('Cancel'),
    onOk: async () => {
      ngxConfigStore.setNgxConfig(r.tokenized)
      // Keep the site file name; regeneration must not rename the site.
      ngxConfig.value.name = siteName.value
      // Select the TLS server so the certificate flow targets the 443 block.
      if (r.tokenized.servers.length > 1)
        ngxConfigStore.curServerIdx = 1
      // In advance mode the editor shows the raw text, keep it in sync.
      if (advanceMode.value)
        configText.value = r.template
      open.value = false
      message.success($gettext('Configuration regenerated'))
    },
  })
}
</script>

<template>
  <AModal
    v-model:open="open"
    :title="$gettext('Quick Setup')"
    :width="640"
    :mask-closable="false"
    :footer="null"
  >
    <AAlert
      v-if="quick.state.enableTLS && editorStore.getTLSServerIssues().length > 0"
      type="warning"
      class="mb-4"
      show-icon
      :message="$gettext('Issue a certificate to enable TLS before saving.')"
    />

    <QuickSetupForm
      :quick="quick"
      :show-name="false"
    />

    <div class="modal-footer mt-4 text-right">
      <ASpace>
        <AButton @click="open = false">
          {{ $gettext('Cancel') }}
        </AButton>
        <AButton
          type="primary"
          :loading="quickGenerating || quickAnalyzing"
          :disabled="!quickFormValid"
          @click="generateConfig"
        >
          {{ $gettext('Generate Config') }}
        </AButton>
      </ASpace>
    </div>
  </AModal>
</template>
