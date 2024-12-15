<script setup lang="ts">
import auth from '@/api/auth'
import install from '@/api/install'
import passkey from '@/api/passkey'
import ICP from '@/components/ICP/ICP.vue'
import SetLanguage from '@/components/SetLanguage/SetLanguage.vue'
import SwitchAppearance from '@/components/SwitchAppearance/SwitchAppearance.vue'
import Authorization from '@/components/TwoFA/Authorization.vue'
import gettext from '@/gettext'
import { useUserStore } from '@/pinia'
import { KeyOutlined, LockOutlined, UserOutlined } from '@ant-design/icons-vue'
import { startAuthentication } from '@simplewebauthn/browser'
import { Form, message } from 'ant-design-vue'

const thisYear = new Date().getFullYear()

const route = useRoute()
const router = useRouter()

install.get_lock().then(async (r: { lock: boolean }) => {
  if (!r.lock)
    await router.push('/install')
})

const loading = ref(false)
const enabled2FA = ref(false)
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
const { login, passkeyLogin } = userStore
const { secureSessionId } = storeToRefs(userStore)

function onSubmit() {
  validate().then(async () => {
    loading.value = true

    await auth.login(modelRef.username, modelRef.password, passcode.value, recoveryCode.value).then(async r => {
      const next = (route.query?.next || '').toString() || '/'
      switch (r.code) {
        case 200:
          message.success($gettext('Login successful'), 1)
          login(r.token)
          await nextTick()
          secureSessionId.value = r.secure_session_id
          await router.push(next)
          break
        case 199:
          enabled2FA.value = true
          break
      }
    }).catch(e => {
      switch (e.code) {
        case 4031:
          message.error($gettext('Incorrect username or password'))
          break
        case 4291:
          message.error($gettext('Too many login failed attempts, please try again later'))
          break
        case 4033:
          message.error($gettext('User is banned'))
          break
        case 4034:
          refOTP.value?.clearInput()
          message.error($gettext('Invalid 2FA or recovery code'))
          break
        default:
          message.error($gettext(e.message ?? 'Server error'))
          break
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
  .catch(e => {
    message.error($gettext(e.message ?? 'Server error'))
  })

function loginWithCasdoor() {
  window.location.href = casdoor_uri.value
}

if (route.query?.code !== undefined && route.query?.state !== undefined) {
  loading.value = true
  auth.casdoor_login(route.query?.code?.toString(), route.query?.state?.toString()).then(async () => {
    message.success($gettext('Login successful'), 1)

    const next = (route.query?.next || '').toString() || '/'

    await router.push(next)
  }).catch(e => {
    message.error($gettext(e.message ?? 'Server error'))
  })
  loading.value = false
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

const passkeyLoginLoading = ref(false)
async function handlePasskeyLogin() {
  passkeyLoginLoading.value = true
  try {
    const begin = await auth.begin_passkey_login()
    const asseResp = await startAuthentication({ optionsJSON: begin.options.publicKey })

    const r = await auth.finish_passkey_login({
      session_id: begin.session_id,
      options: asseResp,
    })

    if (r.token) {
      const next = (route.query?.next || '').toString() || '/'

      passkeyLogin(asseResp.rawId, r.token)
      secureSessionId.value = r.secure_session_id
      await router.push(next)
    }
  }
  // eslint-disable-next-line ts/no-explicit-any
  catch (e: any) {
    message.error($gettext(e.message ?? 'Server error'))
  }
  passkeyLoginLoading.value = false
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
          <AForm id="components-form-demo-normal-login">
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
                html-type="submit"
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
                  :loading="passkeyLoginLoading"
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

    .footer {
      padding: 30px 20px;
      text-align: center;
      font-size: 14px;
    }
  }
}
</style>
