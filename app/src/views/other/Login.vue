<script setup lang="ts">
import { LockOutlined, UserOutlined } from '@ant-design/icons-vue'
import { reactive, ref, watch } from 'vue'
import { useRoute, useRouter } from 'vue-router'
import { Form, message } from 'ant-design-vue'
import gettext from '@/gettext'
import { useUserStore } from '@/pinia'
import auth from '@/api/auth'
import install from '@/api/install'
import SetLanguage from '@/components/SetLanguage/SetLanguage.vue'
import http from '@/lib/http'
import SwitchAppearance from '@/components/SwitchAppearance/SwitchAppearance.vue'

const thisYear = new Date().getFullYear()

const route = useRoute()
const router = useRouter()

install.get_lock().then(async (r: { lock: boolean }) => {
  if (!r.lock)
    await router.push('/install')
})

const { $gettext } = gettext
const loading = ref(false)

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

const onSubmit = () => {
  validate().then(async () => {
    loading.value = true
    // eslint-disable-next-line promise/no-nesting
    await auth.login(modelRef.username, modelRef.password).then(async () => {
      message.success($gettext('Login successful'), 1)

      const next = (route.query?.next || '').toString() || '/'

      await router.push(next)
      // eslint-disable-next-line promise/no-nesting
    }).catch(e => {
      message.error($gettext(e.message ?? 'Server error'))
    })
    loading.value = false
  })
}

const user = useUserStore()

if (user.is_login) {
  const next = (route.query?.next || '').toString() || '/dashboard'

  router.push(next)
}

watch(() => gettext.current, () => {
  clearValidate()
})

const has_casdoor = ref(false)
const casdoor_uri = ref('')

http.get('/casdoor_uri')
  .then(response => {
    if (response?.uri) {
      has_casdoor.value = true
      casdoor_uri.value = response.uri
    }
  })
  .catch(e => {
    message.error($gettext(e.message ?? 'Server error'))
  })

const loginWithCasdoor = () => {
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
            <AFormItem>
              <AButton
                type="primary"
                block
                html-type="submit"
                :loading="loading"
                @click="onSubmit"
              >
                {{ $gettext('Login') }}
              </AButton>
            </AFormItem>
          </AForm>
          <AButton
            v-if="has_casdoor"
            block
            html-type="submit"
            :loading="loading"
            @click="loginWithCasdoor"
          >
            {{ $gettext('SSO Login') }}
          </AButton>
          <div class="footer">
            <p>Copyright © 2020 - {{ thisYear }} Nginx UI</p>
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
    max-width: 400px;
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
      padding: 30px;
      text-align: center;
      font-size: 14px;
    }
  }
}

</style>
