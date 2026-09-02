<script setup lang="ts">
import { ArrowLeftOutlined } from '@antdv-next/icons'
import settingsApi from '@/api/settings'
import { TwoFACancelledError, use2FAModal } from '@/components/TwoFA'
import { getErrorMessage } from '@/lib/http'
import NginxHostSetupWizard from './components/NginxHostSetup/Wizard.vue'
import { applyNginxControlSettings, buildNginxControlPayload } from './nginxControl'
import useSystemSettingsStore from './store'

const systemSettingsStore = useSystemSettingsStore()
const { data } = storeToRefs(systemSettingsStore)
const router = useRouter()
const twoFAModal = use2FAModal()
const { message } = useGlobalApp()

const isAuthorizing = ref(true)
const isReady = ref(false)
const isSaving = ref(false)
const needsTwoFASetup = ref(false)
const loadError = ref('')

function goBack() {
  return router.push({
    path: '/preference',
    query: { tab: 'nginx' },
  })
}

function goToTwoFASettings() {
  return router.push({
    path: '/profile',
    hash: '#two-factor-authentication',
  })
}

async function initialize() {
  isAuthorizing.value = true
  isReady.value = false
  needsTwoFASetup.value = false
  loadError.value = ''

  try {
    const secureSessionID = await twoFAModal.open()
    if (!secureSessionID) {
      needsTwoFASetup.value = true
      return
    }

    const loaded = await systemSettingsStore.getSettings()
    if (!loaded) {
      loadError.value = $gettext('Failed to load settings')
      return
    }
    const nginx = data.value.nginx
    if (nginx.host_mode === 'ssh'
      && nginx.host_access_mode !== 'sftp'
      && nginx.host_access_mode !== 'mounted') {
      loadError.value = $gettext('The saved SSH access mode is missing or invalid. Set host_access_mode to sftp or mounted in the configuration file.')
      return
    }
    isReady.value = true
  }
  catch (error) {
    if (error instanceof TwoFACancelledError) {
      await goBack()
      return
    }
    loadError.value = getErrorMessage(error, $gettext('Failed to authorize protected settings'))
  }
  finally {
    isAuthorizing.value = false
  }
}

async function save() {
  loadError.value = ''
  isSaving.value = true
  try {
    const saved = await settingsApi.saveNginxControl(
      buildNginxControlPayload(data.value.nginx, 'host_via_ssh'),
      { skipErrHandling: true },
    )
    applyNginxControlSettings(data.value.nginx, saved)
    message.success($gettext('Save successfully'))
    await goBack()
  }
  catch (error) {
    loadError.value = getErrorMessage(error, $gettext('Failed to save Nginx control settings'))
    // A dismissed 2FA prompt resolves to an empty message on purpose, and an
    // empty toast would still render an empty notice.
    if (loadError.value) {
      // The alert sits at the top of the page, far from the Save button.
      message.error(loadError.value)
    }
  }
  finally {
    isSaving.value = false
  }
}

onMounted(initialize)
</script>

<template>
  <main class="host-setup-page">
    <div class="mb-5">
      <AButton @click="goBack">
        <ArrowLeftOutlined />
        {{ $gettext('Back to Nginx settings') }}
      </AButton>
    </div>

    <div v-if="isAuthorizing" class="flex min-h-320px items-center justify-center">
      <ASpin size="large" :description="$gettext('Authorizing...')" />
    </div>

    <AResult
      v-else-if="needsTwoFASetup"
      status="403"
      :title="$gettext('Two-factor authentication required')"
      :sub-title="$gettext('Enable two-factor authentication before changing Nginx control settings.')"
    >
      <template #extra>
        <div class="flex flex-wrap justify-center gap-2">
          <AButton type="primary" @click="goToTwoFASettings">
            {{ $gettext('2FA Settings') }}
          </AButton>
          <AButton @click="goBack">
            {{ $gettext('Back') }}
          </AButton>
        </div>
      </template>
    </AResult>

    <AResult
      v-else-if="loadError && !isReady"
      status="error"
      :title="$gettext('Unable to open SSH setup')"
      :sub-title="loadError"
    >
      <template #extra>
        <div class="flex flex-wrap justify-center gap-2">
          <AButton type="primary" @click="initialize">
            {{ $gettext('Retry') }}
          </AButton>
          <AButton @click="goBack">
            {{ $gettext('Back') }}
          </AButton>
        </div>
      </template>
    </AResult>

    <template v-else-if="isReady">
      <AAlert
        v-if="loadError"
        type="error"
        show-icon
        closable
        :title="$gettext('Failed to save Nginx control settings')"
        :description="loadError"
        class="mb-4"
        @close="loadError = ''"
      />
      <NginxHostSetupWizard :saving="isSaving" @save="save" />
    </template>
  </main>
</template>

<style lang="less" scoped>
.host-setup-page {
  width: min(100%, 1180px);
  min-height: 560px;
  margin: 0 auto;
  padding: 8px 12px 32px;
}

@media (max-width: 600px) {
  .host-setup-page {
    padding-right: 10px;
    padding-left: 10px;
  }
}
</style>
