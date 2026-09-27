<script setup lang="ts">
import { Modal } from 'antdv-next'
import site from '@/api/site'
import { useGlobalStore } from '@/pinia'
import { useSiteEditorStore } from '@/views/site/site_edit/components/SiteEditor/store'
import { isHTTPChallengeLocation } from '../../composables/useHTTPSRedirect'
import EditorHTTPSCard from '../HTTPS/EditorHTTPSCard.vue'
import { extractServerDomains, sameStringList } from '../HTTPS/siteHTTPSState'

const props = defineProps<{
  configName: string
}>()

const { message } = useGlobalApp()
const editorStore = useSiteEditorStore()
const { ngxConfig, curServer, curDirectivesMap, autoCert } = storeToRefs(editorStore)

const [modal, ContextHolder] = Modal.useModal()

const noServerName = computed(() => {
  if (!curDirectivesMap.value.server_name)
    return true

  return curDirectivesMap.value.server_name.length === 0
})

watch(noServerName, () => {
  autoCert.value = false
})

const domains = computed<string[]>(previous => {
  const next = extractServerDomains([curServer.value])
  return previous && sameStringList(previous, next) ? previous : next
})

// ---- Enable / reissue ---------------------------------------------------

const modalOpen = ref(false)
const issued = ref(false)
const httpsCard = useTemplateRef('httpsCard')
const httpsRunning = computed(() => httpsCard.value?.running ?? false)

function openHTTPSModal() {
  if (!httpsRunning.value)
    httpsCard.value?.reset()
  issued.value = false
  modalOpen.value = true
}

function onIssued() {
  issued.value = true
}

// The backend rewrote the site file and enabled auto-renewal, so reload the
// site (config and certificate info) once the operator closes the dialog.
watch(modalOpen, open => {
  if (!open && issued.value) {
    issued.value = false
    editorStore.init(props.configName)
  }
})

// ---- Disable auto-renewal -----------------------------------------------

const disabling = ref(false)

async function disableAutoCert() {
  disabling.value = true
  try {
    ngxConfig.value.servers.forEach(server => {
      server.locations = server.locations?.filter(location => !isHTTPChallengeLocation(location))
    })
    // Skip syncing the response: the certificate record still has auto-renewal
    // enabled until remove_auto_cert returns; the reload below picks up both.
    await editorStore.save({ syncResponse: false })
    await site.remove_auto_cert(props.configName)
    message.success($gettext('Auto-renewal disabled for %{name}', { name: props.configName }))
  }
  catch (e) {
    const error = e as { message?: string }
    message.error(error?.message ?? $gettext('Disable auto-renewal failed for %{name}', { name: props.configName }))
  }
  finally {
    disabling.value = false
  }

  await editorStore.init(props.configName)
}

function onchange() {
  if (!autoCert.value) {
    openHTTPSModal()
    return
  }

  modal.confirm({
    title: $gettext('Do you want to disable auto-cert renewal?'),
    content: $gettext('We will remove the HTTPChallenge configuration from '
      + 'this file and reload the Nginx. Are you sure you want to continue?'),
    okText: $gettext('OK'),
    cancelText: $gettext('Cancel'),
    mask: false,
    centered: true,
    onOk: disableAutoCert,
  })
}

const globalStore = useGlobalStore()
const { processingStatus } = storeToRefs(globalStore)
</script>

<template>
  <div>
    <ContextHolder />
    <AModal
      v-model:open="modalOpen"
      :title="$gettext('Obtain certificate')"
      :mask-closable="false"
      :closable="!httpsRunning"
      :keyboard="!httpsRunning"
      :footer="null"
      :width="640"
    >
      <!-- This dialog obtains a Let's Encrypt certificate; picking an existing
           one is Change Certificate's job. -->
      <EditorHTTPSCard
        ref="httpsCard"
        compact
        :existing-certificate="false"
        :domains
        @success="onIssued"
      >
        <template #actions>
          <AButton
            v-if="issued"
            type="primary"
            @click="modalOpen = false"
          >
            {{ $gettext('Done') }}
          </AButton>
        </template>
      </EditorHTTPSCard>
    </AModal>
    <div class="issue-cert">
      <AFormItem :label="$gettext('Encrypt website with Let\'s Encrypt')">
        <ASwitch
          :loading="disabling"
          :checked="autoCert"
          :disabled="noServerName || processingStatus.auto_cert_processing"
          @change="onchange"
        />
        <span v-if="processingStatus.auto_cert_processing" class="ml-4">
          {{ $gettext('AutoCert is running, please wait...') }}
        </span>
      </AFormItem>
    </div>
  </div>
</template>

<style lang="less" scoped>
.issue-cert {
  margin: 15px 0;
}
</style>
