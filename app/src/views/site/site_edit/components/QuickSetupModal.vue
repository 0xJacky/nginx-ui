<script setup lang="ts">
import type { NgxConfig } from '@/api/ngx'
import template from '@/api/template'
import { useNgxConfigStore } from '@/components/NgxConfigEditor'
import { ConfigStatus } from '@/constants'
import QuickSetupForm from '@/views/site/components/QuickSetup/QuickSetupForm.vue'
import { useQuickConfig } from '@/views/site/components/QuickSetup/useQuickConfig'
import { isCompleteTLSServer } from '../composables/useHTTPSRedirect'
import { carryOverCertificate, hasPendingTLSServer } from './HTTPS/siteHTTPSState'
import { useSiteEditorStore } from './SiteEditor/store'

const emit = defineEmits<{
  // The config was saved without its pending TLS server; the page should
  // lead the operator to the HTTPS card.
  httpsPending: []
}>()

const open = defineModel<boolean>('open', { default: false })

const { message, modal } = useGlobalApp()
const route = useRoute()

const siteName = computed(() => decodeURIComponent(route.params?.name?.toString() ?? ''))

const editorStore = useSiteEditorStore()
const { advanceMode, configText, data } = storeToRefs(editorStore)

const ngxConfigStore = useNgxConfigStore()
const { ngxConfig } = storeToRefs(ngxConfigStore)

const quick = useQuickConfig()
const { quickFormValid, quickGenerating } = quick
const quickAnalyzing = ref(false)
const quickSaving = ref(false)

const isSiteActive = computed(() => data.value.status === ConfigStatus.Enabled
  || data.value.status === ConfigStatus.Maintenance)

// An active site cannot hold a TLS server without certificate (nginx -t
// rejects it), so in basic mode the regenerated config is saved right away
// without it, like the wizard's draft, and the HTTPS card issues the
// certificate. Advanced mode edits raw text, which the card does not handle.
function stagesPendingTLS(config: NgxConfig) {
  return !advanceMode.value && isSiteActive.value && hasPendingTLSServer(config)
}

// Shown up front, before the config is generated.
const willStageTLS = computed(() => quick.state.enableTLS
  && quick.state.type !== 'redirect'
  && !advanceMode.value
  && isSiteActive.value
  && !ngxConfig.value.servers?.some(isCompleteTLSServer))

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

async function applyStaged(config: NgxConfig) {
  quickSaving.value = true
  try {
    ngxConfigStore.setNgxConfig(config)
    ngxConfig.value.name = siteName.value
    await editorStore.save({ omitIncompleteTLSServers: true })
    open.value = false
    message.success($gettext('Configuration saved. Enable HTTPS from the HTTPS card.'))
    emit('httpsPending')
  }
  catch {
    // The request layer reports the error; the generated config stays in the
    // editor so the operator can fix and save it.
    open.value = false
  }
  finally {
    quickSaving.value = false
  }
}

async function generateConfig() {
  const r = await quick.generate()

  // A site that already serves HTTPS keeps its certificate instead of getting
  // an empty TLS server. Advanced mode uses the raw template text instead.
  if (!advanceMode.value)
    carryOverCertificate(r.tokenized, ngxConfig.value)

  const staged = stagesPendingTLS(r.tokenized)

  modal.confirm({
    title: $gettext('Replace configuration?'),
    content: staged
      ? $gettext('The generated configuration will replace the current one and is saved right away without its HTTPS server. Any custom directives or locations will be lost.')
      : $gettext('The generated configuration will replace the current one. Any custom directives or locations will be lost.'),
    okText: staged ? $gettext('Replace and save') : $gettext('Replace'),
    cancelText: $gettext('Cancel'),
    onOk: async () => {
      if (staged) {
        await applyStaged(r.tokenized)
        return
      }

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
      v-if="willStageTLS"
      type="info"
      class="mb-4"
      show-icon
      :title="$gettext('The configuration is saved without its HTTPS server first. Then enable HTTPS from the HTTPS card on this page.')"
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
          :loading="quickGenerating || quickAnalyzing || quickSaving"
          :disabled="!quickFormValid"
          @click="generateConfig"
        >
          {{ $gettext('Generate Config') }}
        </AButton>
      </ASpace>
    </div>
  </AModal>
</template>
