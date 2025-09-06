<script setup lang="ts">
import { KeyOutlined, LoadingOutlined, LockOutlined, UserOutlined } from '@ant-design/icons-vue'
import { startAuthentication } from '@simplewebauthn/browser'
import { Form } from 'ant-design-vue'
import auth from '@/api/auth'
import install from '@/api/install'
import passkey from '@/api/passkey'
import ICP from '@/components/ICP'
import SetLanguage from '@/components/SetLanguage'
import SwitchAppearance from '@/components/SwitchAppearance'
import Authorization from '@/components/TwoFA'
import gettext from '@/gettext'
import { useSettingsStore, useUserStore } from '@/pinia'

const thisYear = new Date().getFullYear()

const route = useRoute()
const router = useRouter()

const loading = ref(false)
const { message } = useGlobalApp()
const enabled2FA = ref(false)

const isDebugMode = computed(() => route.query.debug === 'true')

const loadingIndicator = h(LoadingOutlined, {
  style: {
    fontSize: '32px',
    color: '#1890ff',
  },
  spin: true,
})

function simulateLoading() {
  loading.value = true
  setTimeout(() => {
    loading.value = false
  }, 3000)
}

function simulate2FA() {
  enabled2FA.value = !enabled2FA.value
}

function toggleDebugLoading() {
  loading.value = !loading.value
}

install.get_lock().then(async (r: { lock: boolean }) => {
  if (!r.lock)
    await router.push('/install')
})
const refOTP = useTemplateRef('refOTP')
const passcode = ref('')
const recoveryCode = ref('')
const passkeyConfigStatus = ref(false)

const modelRef = reactive({
  username: '',
  password: '',
})

const rulesRef = reactive({
  username: [
    {
      required: true,
      message: () => $gettext('Please input your username!'),
    },
  ],
  password: [
    {
      required: true,
      message: () => $gettext('Please input your password!'),
    },
  ],
})

const { validate, validateInfos, clearValidate } = Form.useForm(modelRef, rulesRef)
const userStore = useUserStore()
const settingsStore = useSettingsStore()
const { login, passkeyLogin } = userStore
const { secureSessionId } = storeToRefs(userStore)

interface LoginSuccessOptions {
  token?: string
  shortToken?: string
  secureSessionId?: string
  loginType?: 'normal' | 'passkey'
  passkeyRawId?: string
  showSuccessMessage?: boolean
}

async function handleLoginSuccess(options: LoginSuccessOptions = {}) {
  const {
    token,
    shortToken,
    secureSessionId: sessionId,
    loginType = 'normal',
    passkeyRawId,
    showSuccessMessage = true,
  } = options

  if (showSuccessMessage) {
    message.success($gettext('Login successful'), 1)
  }

  // Handle different login types
  if (loginType === 'passkey' && passkeyRawId && token) {
    passkeyLogin(passkeyRawId, token, shortToken)
  }
  else if (token) {
    login(token, shortToken)
  }

  await nextTick()

  if (sessionId) {
    secureSessionId.value = sessionId
  }

  await userStore.getCurrentUser()
  await nextTick()
  if (gettext.current !== 'en' && gettext.current !== userStore.info?.language) {
    await userStore.updateCurrentUserLanguage(gettext.current)
  }
  else {
    await settingsStore.set_language(userStore.info?.language)
  }

  const next = (route.query?.next || '').toString() || '/'
  await router.push(next)
}

function onSubmit() {
  validate().then(async () => {
    loading.value = true

    await auth.login(modelRef.username, modelRef.password, passcode.value, recoveryCode.value).then(async r => {
      switch (r.code) {
        case 200:
          await handleLoginSuccess({
            token: r.token,
            shortToken: r.short_token,
            secureSessionId: r.secure_session_id,
          })
          break
        case 199:
          enabled2FA.value = true
          break
      }
    }).catch(e => {
      if (e.code === 4043) {
        refOTP.value?.clearInput()
      }
    })
    loading.value = false
  })
}

const user = useUserStore()

if (user.isLogin) {
  const next = (route.query?.next || '').toString() || '/dashboard'

  router.push(next)
}

watch(() => gettext.current, () => {
  clearValidate()
})

const has_casdoor = ref(false)
const casdoor_uri = ref('')

auth.get_casdoor_uri()
  .then(r => {
    if (r?.uri) {
      has_casdoor.value = true
      casdoor_uri.value = r.uri
    }
  })

function loginWithCasdoor() {
  window.location.href = casdoor_uri.value
}

if (route.query?.code !== undefined && route.query?.state !== undefined) {
  loading.value = true
  auth.casdoor_login(route.query?.code?.toString(), route.query?.state?.toString()).then(async () => {
    await handleLoginSuccess()
  }).finally(() => {
    loading.value = false
  })
}

function handleOTPSubmit(code: string, recovery: string) {
  passcode.value = code
  recoveryCode.value = recovery

  nextTick(() => {
    onSubmit()
  })
}

passkey.get_config_status().then(r => {
  passkeyConfigStatus.value = r.status
})

async function handlePasskeyLogin() {
  loading.value = true

  try {
    const begin = await auth.begin_passkey_login()
    const asseResp = await startAuthentication({ optionsJSON: begin.options.publicKey })

    const r = await auth.finish_passkey_login({
      session_id: begin.session_id,
      options: asseResp,
    })

    if (r.token) {
      await handleLoginSuccess({
        token: r.token,
        shortToken: r.short_token,
        secureSessionId: r.secure_session_id,
        loginType: 'passkey',
        passkeyRawId: asseResp.rawId,
      })
    }
  }
  catch (e) {
    console.error(e)
  }
  finally {
    loading.value = false
  }
}
</script>

<template>
  <ALayout>
    <ALayoutContent>
      <div class="login-container">
        <div class="login-form">
          <div class="project-title">
            <h1>Nginx UI</h1>
          </div>

          <div v-if="loading" class="loading-container">
            <ASpin :indicator="loadingIndicator" />
            <div class="loading-text">
              {{ $gettext('Authenticating...') }}
            </div>
          </div>

          <AForm v-else id="components-form-demo-normal-login">
            <template v-if="!enabled2FA">
              <AFormItem v-bind="validateInfos.username">
                <AInput
                  v-model:value="modelRef.username"
                  :placeholder="$gettext('Username')"
                >
                  <template #prefix>
                    <UserOutlined style="color: rgba(0, 0, 0, 0.25)" />
                  </template>
                </AInput>
              </AFormItem>
              <AFormItem v-bind="validateInfos.password">
                <AInputPassword
                  v-model:value="modelRef.password"
                  :placeholder="$gettext('Password')"
                >
                  <template #prefix>
                    <LockOutlined style="color: rgba(0, 0, 0, 0.25)" />
                  </template>
                </AInputPassword>
              </AFormItem>
              <AButton
                v-if="has_casdoor"
                block
                :loading="loading"
                class="mb-5"
                @click="loginWithCasdoor"
              >
                {{ $gettext('SSO Login') }}
              </AButton>
            </template>
            <div v-else>
              <Authorization
                ref="refOTP"
                :two-f-a-status="{
                  enabled: true,
                  otp_status: true,
                  passkey_status: false,
                  recovery_codes_generated: true,
                }"
                @submit-o-t-p="handleOTPSubmit"
              />
            </div>

            <AFormItem v-if="!enabled2FA">
              <AButton
                type="primary"
                block
                html-type="submit"
                :loading="loading"
                class="mb-2"
                @click="onSubmit"
              >
                {{ $gettext('Login') }}
              </AButton>

              <div
                v-if="passkeyConfigStatus"
                class="flex flex-col justify-center"
              >
                <ADivider>
                  <div class="text-sm font-normal opacity-75">
                    {{ $gettext('Or') }}
                  </div>
                </ADivider>

                <AButton
                  :disabled="loading"
                  @click="handlePasskeyLogin"
                >
                  <KeyOutlined />
                  {{ $gettext('Sign in with a passkey') }}
                </AButton>
              </div>
            </AFormItem>
          </AForm>
          <div class="footer">
            <p class="mb-4">
              Copyright © 2021 - {{ thisYear }} Nginx UI
            </p>
            <ICP class="mb-4" />
            Language
            <SetLanguage class="inline" />
            <div class="flex justify-center mt-4">
              <SwitchAppearance />
            </div>
          </div>
        </div>

        <!-- Debug Panel -->
        <div v-if="isDebugMode" class="debug-panel">
          <div class="debug-header">
            🐛 Debug Panel
          </div>
          <div class="debug-buttons">
            <AButton size="small" @click="toggleDebugLoading">
              {{ loading ? 'Stop Loading' : 'Toggle Loading' }}
            </AButton>
            <AButton size="small" @click="simulateLoading">
              Simulate 3s Loading
            </AButton>
            <AButton size="small" @click="simulate2FA">
              {{ enabled2FA ? 'Hide 2FA' : 'Show 2FA' }}
            </AButton>
          </div>
        </div>
      </div>
    </ALayoutContent>
  </ALayout>
</template>

<style lang="less" scoped>
.ant-layout-content {
  background: #fff;
}

.dark .ant-layout-content {
  background: transparent;
}

.login-container {
  display: flex;
  align-items: center;
  justify-content: center;
  height: 100vh;

  .login-form {
    max-width: 420px;
    width: 80%;

    .project-title {
      margin: 50px;

      h1 {
        font-size: 50px;
        font-weight: 100;
        text-align: center;
      }
    }

    .anticon {
      color: #a8a5a5 !important;
    }

    .loading-container {
      display: flex;
      flex-direction: column;
      align-items: center;
      justify-content: center;
      padding: 80px 20px;
      text-align: center;

      .loading-text {
        margin-top: 16px;
        font-size: 16px;
        color: rgba(0, 0, 0, 0.65);
      }
    }

    .dark .loading-container .loading-text {
      color: rgba(255, 255, 255, 0.65);
    }

    .footer {
      padding: 30px 20px;
      text-align: center;
      font-size: 14px;
    }
  }
}

.debug-panel {
  position: fixed;
  bottom: 20px;
  left: 50%;
  transform: translateX(-50%);
  background: rgba(0, 0, 0, 0.8);
  color: white;
  padding: 12px 16px;
  border-radius: 8px;
  box-shadow: 0 4px 12px rgba(0, 0, 0, 0.3);
  z-index: 1000;
  backdrop-filter: blur(8px);

  .debug-header {
    font-size: 12px;
    font-weight: 500;
    margin-bottom: 8px;
    text-align: center;
  }

  .debug-buttons {
    display: flex;
    gap: 8px;
    flex-wrap: wrap;
    justify-content: center;
  }
}

.dark .debug-panel {
  background: rgba(255, 255, 255, 0.15);
  color: rgba(255, 255, 255, 0.9);
}
</style>
