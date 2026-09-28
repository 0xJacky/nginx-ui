<script setup lang="ts">
import { HistoryOutlined, ThunderboltOutlined } from '@antdv-next/icons'
import { AccessControlCard, LocationAccessSelect, LocationAccessTag, useAccessControlStore } from '@/components/AccessControl'
import CodeEditor from '@/components/CodeEditor/CodeEditor.vue'
import ConfigHistory from '@/components/ConfigHistory'
import FooterToolBar from '@/components/FooterToolbar'
import InspectConfig from '@/components/InspectConfig'
import NgxConfigEditor from '@/components/NgxConfigEditor'
import { siteUpstreamContextKey } from '@/components/NgxConfigEditor/siteUpstreamContext'
import UpstreamCards from '@/components/UpstreamCards/UpstreamCards.vue'
import { ConfigStatus } from '@/constants'
import Cert from '@/views/site/site_edit/components/Cert'
import EnableTLS from '@/views/site/site_edit/components/EnableTLS'
import EditorHTTPSCard from '@/views/site/site_edit/components/HTTPS/EditorHTTPSCard.vue'
import { extractSiteDomains, hasTLSServer, sameStringList } from '@/views/site/site_edit/components/HTTPS/siteHTTPSState'
import QuickSetupModal from '@/views/site/site_edit/components/QuickSetupModal.vue'
import { useSiteEditorStore } from './store'

const { message } = App.useApp()

const route = useRoute()

const name = computed(() => decodeURIComponent(route.params?.name?.toString() ?? ''))

const editorStore = useSiteEditorStore()
const {
  data,
  parseErrorStatus,
  parseErrorMessage,
  filepath,
  configText,
  loading,
  saving,
  certInfoMap,
  advanceMode,
  curSupportSSL,
  dnsLinked,
  linkedDNSName,
  ngxConfig,
} = storeToRefs(editorStore)

// A site without any TLS server gets the HTTPS onboarding card; TLS servers
// are handled per server tab by the Cert component.
const showHTTPSOnboarding = computed(() => {
  const status = data.value.status
  return (status === ConfigStatus.Enabled || status === ConfigStatus.Maintenance)
    && (ngxConfig.value.servers?.length ?? 0) > 0
    && !hasTLSServer(ngxConfig.value)
})

// Keep the same array while the names are unchanged so the card does not
// reset domains the operator added.
const siteDomains = computed<string[]>(previous => {
  const next = extractSiteDomains(ngxConfig.value)
  return previous && sameStringList(previous, next) ? previous : next
})

const httpsCardWrapper = useTemplateRef('httpsCardWrapper')

// Quick setup saved the site without its pending TLS server; bring the card
// that finishes HTTPS into view once it renders.
async function focusHTTPSCard() {
  await nextTick()
  httpsCardWrapper.value?.scrollIntoView({ behavior: 'smooth', block: 'start' })
}

function onHTTPSEnabled() {
  // The backend rewrote the site file; reload config and certificate info.
  editorStore.init(name.value)
}

// Provide DNS link status to child components
provide('dnsLinked', dnsLinked)
provide('linkedDNSName', linkedDNSName)

// Lets the Upstream section turn one of this site's upstream blocks into a
// shared group; that rewrites the site file, so the editor saves or drops its
// changes first and reloads the file afterwards.
provide(siteUpstreamContextKey, {
  siteName: name,
  hasUnsavedChanges: () => editorStore.hasUnsavedChanges(),
  async save() {
    await editorStore.save()
    message.success($gettext('Saved successfully'))
  },
  reload: () => editorStore.init(name.value),
})

// Get upstream targets from backend API data
const upstreamTargets = computed(() => {
  return data.value.proxy_targets || []
})

const showHistory = ref(false)

const quickSetupOpen = ref(false)

// Use Vue 3.4+ useTemplateRef for InspectConfig component
const inspectConfigRef = useTemplateRef<InstanceType<typeof InspectConfig>>('inspectConfig')

// The access control card, the location selects and the right panel read the
// access state of the open site from this store.
const accessControl = useAccessControlStore()
watch(advanceMode, value => {
  accessControl.advanced = value
}, { immediate: true })
onMounted(() => accessControl.activate())
onBeforeUnmount(() => accessControl.deactivate())

// Vue Router can reuse this component when only the site name changes, so reload
// on the route parameter instead of on mount alone.
watch(name, value => {
  editorStore.init(value)
}, { immediate: true })

async function save() {
  try {
    await editorStore.save()
    message.success($gettext('Saved successfully'))
    // Run test after saving to verify configuration
    inspectConfigRef.value?.test()
  }
  catch {
    // do nothing
  }
}
</script>

<template>
  <!-- The body style goes through the semantic prop: a :deep(.ant-card-body)
       rule would also strip the padding of every card nested in the editor,
       such as the HTTPS card. -->
  <ACard
    class="site-edit-container overflow-hidden"
    variant="borderless"
    :styles="{ body: { maxHeight: '100%', overflowY: 'scroll', padding: 0 } }"
  >
    <template #title>
      <span style="margin-right: 10px">{{ $gettext('Edit %{n}', { n: name }) }}</span>
      <ATag
        v-if="data.status === ConfigStatus.Enabled"
        color="blue"
      >
        {{ $gettext('Enabled') }}
      </ATag>
      <ATag
        v-else-if="data.status === ConfigStatus.Disabled"
        color="red"
      >
        {{ $gettext('Disabled') }}
      </ATag>
      <ATag
        v-else-if="data.status === ConfigStatus.Maintenance"
        color="orange"
      >
        {{ $gettext('Maintenance') }}
      </ATag>
    </template>
    <template #extra>
      <ASpace>
        <AButton
          v-if="filepath"
          type="link"
          @click="showHistory = true"
        >
          <template #icon>
            <HistoryOutlined />
          </template>
          {{ $gettext('History') }}
        </AButton>
        <AButton
          type="link"
          @click="quickSetupOpen = true"
        >
          <template #icon>
            <ThunderboltOutlined />
          </template>
          {{ $gettext('Quick Setup') }}
        </AButton>
        <div class="mode-switch">
          <div class="switch">
            <ASwitch
              size="small"
              :disabled="parseErrorStatus"
              :checked="advanceMode"
              :loading="loading"
              @change="editorStore.handleModeChange"
            />
          </div>
          <template v-if="advanceMode">
            <div>{{ $gettext('Advance Mode') }}</div>
          </template>
          <template v-else>
            <div>{{ $gettext('Basic Mode') }}</div>
          </template>
        </div>
      </ASpace>
    </template>

    <InspectConfig
      ref="inspectConfig"
      class="mb-0!"
      banner
      :namespace-id="data.namespace_id"
    />

    <div class="card-body">
      <Transition name="slide-fade">
        <div
          v-if="advanceMode"
          key="advance"
        >
          <div
            v-if="parseErrorStatus"
            class="parse-error-alert-wrapper"
          >
            <AAlert
              banner
              :title="$gettext('Nginx Configuration Parse Error')"
              :description="parseErrorMessage"
              type="error"
              show-icon
            />
          </div>
          <div>
            <CodeEditor
              v-model:content="configText"
              no-border-radius
            />
          </div>
        </div>

        <div
          v-else
          key="basic"
          class="domain-edit-container"
        >
          <!-- One entry point for a site without TLS: the HTTPS card, which
               also covers using an existing certificate. -->
          <div v-if="showHTTPSOnboarding" ref="httpsCardWrapper" class="mb-4 px-6">
            <EditorHTTPSCard
              compact
              :domains="siteDomains"
              @success="onHTTPSEnabled"
            />
          </div>
          <EnableTLS v-else />

          <!-- Upstream Cards Display -->
          <UpstreamCards
            :targets="upstreamTargets"
            :namespace-id="data.namespace_id"
          />

          <NgxConfigEditor
            :cert-info="certInfoMap"
            :status="data.status"
          >
            <template #tab-content="{ tabIdx }">
              <Cert
                v-if="curSupportSSL"
                class="mb-4"
                :site-status="data.status"
                :config-name="name"
                :cert-info="certInfoMap?.[tabIdx]"
              />
              <AccessControlCard :server-idx="tabIdx" />
            </template>
            <template #location-label="{ serverIdx, locationIdx }">
              <LocationAccessTag :server-idx="serverIdx" :location-idx="locationIdx" />
            </template>
            <template #location-form="{ serverIdx, locationIdx }">
              <LocationAccessSelect :server-idx="serverIdx" :location-idx="locationIdx" />
            </template>
          </NgxConfigEditor>
        </div>
      </Transition>
    </div>

    <FooterToolBar>
      <ASpace>
        <AButton @click="$router.push('/sites/list')">
          {{ $gettext('Back') }}
        </AButton>
        <AButton
          type="primary"
          :loading="saving"
          @click="save"
        >
          {{ $gettext('Save') }}
        </AButton>
      </ASpace>
    </FooterToolBar>

    <ConfigHistory
      v-model:visible="showHistory"
      v-model:current-content="configText"
      :filepath="filepath"
    />

    <QuickSetupModal
      v-model:open="quickSetupOpen"
      @https-pending="focusHTTPSCard"
    />
  </ACard>
</template>

<style lang="less" scoped>
.mode-switch {
  display: flex;

  .switch {
    display: flex;
    align-items: center;
    margin-right: 5px;
  }
}

.domain-edit-container {
  max-width: 800px;
  margin: 0 auto;
  padding: 24px 0;
}

.site-edit-container {
  height: 100%;
}

.domain-edit-container {
  max-width: 800px;
  margin: 0 auto;
}

.slide-fade-enter-active {
  transition: all .3s ease-in-out;
}

.slide-fade-leave-active {
  transition: all .3s cubic-bezier(1.0, 0.5, 0.8, 1.0);
}

.slide-fade-enter-from, .slide-fade-enter-to, .slide-fade-leave-to {
  transform: translateX(10px);
  opacity: 0;
}

:deep(.tab-content) {
  padding-bottom: 24px;
}
</style>
