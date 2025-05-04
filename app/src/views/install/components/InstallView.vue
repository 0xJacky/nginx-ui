<script setup lang="ts">
import install from '@/api/install'
import SelfCheck, { useSelfCheckStore } from '@/components/SelfCheck'
import SystemRestoreContent from '@/components/SystemRestore'
import { message } from 'ant-design-vue'
import InstallFooter from './InstallFooter.vue'
import InstallForm from './InstallForm.vue'
import InstallHeader from './InstallHeader.vue'
import TimeoutAlert from './TimeoutAlert.vue'

const installTimeout = ref(false)
const activeTab = ref('1')
const step = ref(1)
const selfCheckStore = useSelfCheckStore()
const { hasError } = storeToRefs(selfCheckStore)

const router = useRouter()

function init() {
  install.get_lock().then(async r => {
    if (r.lock)
      await router.push('/login')

    if (r.timeout) {
      installTimeout.value = true
    }
  })
}

if (import.meta.env.DEV) {
  const route = useRoute()
  if (route.query.install !== 'false') {
    init()
  }
  else {
    installTimeout.value = route.query.timeout === 'true'
  }
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

const canProceed = computed(() => {
  return !installTimeout.value && !hasError.value
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
            <SelfCheck class="mb-4" />
            <div class="flex justify-center">
              <AButton v-if="canProceed" type="primary" @click="step = 2">
                {{ $gettext('Next') }}
              </AButton>
              <AAlert
                v-else
                type="error"
                class="mt-4"
                :message="$gettext('Please resolve all issues before proceeding with installation')"
                show-icon
              />
            </div>
          </div>

          <ATabs v-if="step === 2" v-model:active-key="activeTab" class="max-w-400px mx-auto">
            <ATabPane key="1" :tab="$gettext('New Installation')">
              <InstallForm />
            </ATabPane>
            <ATabPane key="2" :tab="$gettext('Restore from Backup')">
              <SystemRestoreContent
                :show-title="false"
                @restore-success="handleRestoreSuccess"
              />
            </ATabPane>
          </ATabs>
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
