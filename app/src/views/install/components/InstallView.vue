<script setup lang="ts">
import install from '@/api/install'
import SelfCheck, { useSelfCheckStore } from '@/components/SelfCheck'
import SystemRestoreContent from '@/components/SystemRestore'
import InstallFooter from './InstallFooter.vue'
import InstallForm from './InstallForm.vue'
import InstallHeader from './InstallHeader.vue'
import TimeoutAlert from './TimeoutAlert.vue'

const installTimeout = ref(false)
const activeTab = ref('1')
const step = ref(1)
const installSecret = ref('')
const authorizedSecret = ref('')
const debugAccessError = ref('')
const { message } = useGlobalApp()
const selfCheckStore = useSelfCheckStore()
const { hasError, loading, checked, accessError } = storeToRefs(selfCheckStore)

const route = useRoute()
const router = useRouter()

function getRouteQueryValue(value: string | string[] | null | undefined): string {
  if (Array.isArray(value)) {
    return value[0] ?? ''
  }

  return value ?? ''
}

const installMode = computed(() => getRouteQueryValue(route.query.install as string | string[] | undefined))
const isInstallDebugMode = computed(() => import.meta.env.DEV && ['false', 'frontend'].includes(installMode.value))
const isFrontendDebugMode = computed(() => import.meta.env.DEV && installMode.value === 'frontend')
const frontendDebugSecret = computed(() => getRouteQueryValue(route.query.debug_secret as string | string[] | undefined) || 'debug-secret')
const routeInstallTimeout = computed(() => getRouteQueryValue(route.query.timeout as string | string[] | undefined) === 'true')

function init() {
  install.get_lock().then(async r => {
    if (r.lock)
      await router.push('/login')

    if (r.timeout) {
      installTimeout.value = true
    }
  })
}

if (isInstallDebugMode.value) {
  installTimeout.value = routeInstallTimeout.value
}
else {
  init()
}

function handleRestoreSuccess(options: { restoreNginx: boolean, restoreNginxUI: boolean }): void {
  message.success($gettext('System restored successfully.'))

  // Only redirect to login page if Nginx UI was restored
  if (options.restoreNginxUI) {
    message.info($gettext('Please log in.'))
    window.location.reload()
  }
}

const trimmedInstallSecret = computed(() => installSecret.value.trim())
const isSetupAuthorized = computed(() => {
  return !!trimmedInstallSecret.value
    && authorizedSecret.value === trimmedInstallSecret.value
    && !debugAccessError.value
    && !accessError.value
})

async function verifyInstallSecret() {
  if (!trimmedInstallSecret.value) {
    return
  }

  debugAccessError.value = ''

  if (isFrontendDebugMode.value) {
    if (trimmedInstallSecret.value !== frontendDebugSecret.value) {
      authorizedSecret.value = ''
      debugAccessError.value = $gettext('Invalid debug secret')
      return
    }

    authorizedSecret.value = trimmedInstallSecret.value
    await selfCheckStore.runCheck({
      setupAuth: true,
      installSecret: trimmedInstallSecret.value,
      debugMode: 'frontend',
    })
    return
  }

  await selfCheckStore.runCheck({
    setupAuth: true,
    installSecret: trimmedInstallSecret.value,
  })

  if (!accessError.value) {
    authorizedSecret.value = trimmedInstallSecret.value
  }
}

watch(trimmedInstallSecret, secret => {
  if (secret !== authorizedSecret.value) {
    authorizedSecret.value = ''
    debugAccessError.value = ''
    activeTab.value = '1'
    step.value = 1
  }
})

const canProceed = computed(() => {
  return isSetupAuthorized.value && checked.value && !installTimeout.value && !hasError.value && !loading.value
})

const blockedMessage = computed(() => {
  if (!trimmedInstallSecret.value) {
    return $gettext('Please enter the install secret before continuing')
  }

  if (debugAccessError.value) {
    return debugAccessError.value
  }

  if (!isSetupAuthorized.value) {
    return $gettext('Please verify the install secret before continuing')
  }

  if (!checked.value) {
    return $gettext('Please complete the system check first')
  }

  return $gettext('Please resolve all issues before proceeding with installation')
})

const steps = computed(() => {
  return [
    {
      title: $gettext('System Check'),
      description: $gettext('Verify system requirements'),
    },
    {
      title: $gettext('Installation'),
      description: $gettext('Setup your Nginx UI'),
    },
  ]
})
</script>

<template>
  <ALayout>
    <ALayoutContent>
      <div class="login-container">
        <InstallHeader />

        <TimeoutAlert class="timeout-alert" :show="installTimeout" />

        <div v-if="!installTimeout" class="install-form">
          <div v-if="!isSetupAuthorized" class="max-w-400px mx-auto">
            <AAlert
              show-icon
              type="info"
              class="mb-4"
              :message="isFrontendDebugMode
                ? $gettext('Frontend debug mode is active. Enter the debug secret to unlock the mock installation flow without sending backend requests.')
                : $gettext('Enter the one-time install secret shown by the install script or found in the config directory hidden file to unlock setup.')"
            />
            <AInputPassword
              v-model:value="installSecret"
              class="mb-4"
              :placeholder="$gettext('Install Secret (*)')"
              @press-enter="verifyInstallSecret"
            />
            <AButton
              block
              type="primary"
              :loading="loading"
              :disabled="!trimmedInstallSecret"
              @click="verifyInstallSecret"
            >
              {{ $gettext('Verify Secret') }}
            </AButton>
            <AAlert
              v-if="debugAccessError || accessError"
              type="error"
              show-icon
              class="mt-4"
              :message="debugAccessError || accessError"
            />
          </div>

          <template v-else>
            <ASteps
              :current="step - 1"
              class="mb-6"
            >
              <AStep
                v-for="(item, index) in steps"
                :key="index"
                :title="item.title"
                :description="item.description"
              />
            </ASteps>

            <div v-if="step === 1">
              <SelfCheck class="mb-4" setup-auth :install-secret="installSecret" :frontend-debug="isFrontendDebugMode" />
              <div class="flex justify-center">
                <AButton v-if="canProceed" type="primary" @click="step = 2">
                  {{ $gettext('Next') }}
                </AButton>
                <AAlert
                  v-else
                  type="error"
                  class="mt-4"
                  :message="blockedMessage"
                  show-icon
                />
              </div>
            </div>

            <ATabs v-if="step === 2" v-model:active-key="activeTab" class="max-w-400px mx-auto">
              <ATabPane key="1" :tab="$gettext('New Installation')">
                <InstallForm :install-secret="installSecret" :frontend-debug="isFrontendDebugMode" />
              </ATabPane>
              <ATabPane key="2" :tab="$gettext('Restore from Backup')">
                <SystemRestoreContent
                  :install-secret="installSecret"
                  setup-auth
                  :frontend-debug="isFrontendDebugMode"
                  :show-title="false"
                  @restore-success="handleRestoreSuccess"
                />
              </ATabPane>
            </ATabs>
          </template>
        </div>

        <InstallFooter />
      </div>
    </ALayoutContent>
  </ALayout>
</template>

<style lang="less" scoped>
.ant-layout-content {
  background: #fff;
}

:deep(.ant-tabs-nav-wrap) {
  justify-content: center;
}

.dark .ant-layout-content {
  background: transparent;
}

.login-container {
  display: flex;
  flex-direction: column;
  justify-content: center;
  align-items: center;
  min-height: 100vh;

  .install-form {
    width: 100%;
    max-width: 800px;
    margin: 0 auto;

    .anticon {
      color: #a8a5a5 !important;
    }
  }
}

.timeout-alert {
  max-width: 400px;
  margin: 0 auto;
}
</style>
