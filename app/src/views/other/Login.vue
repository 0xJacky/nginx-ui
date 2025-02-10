<script setup lang="ts">
import { ref, reactive } from 'vue'
import { Form } from 'ant-design-vue'
import { UserOutlined, LockOutlined, SafetyCertificateOutlined } from '@ant-design/icons-vue'
import auth from '@/api/auth'
import install from '@/api/install'
import passkey from '@/api/passkey'
import { useUserStore } from '@/pinia'
import { useRoute, useRouter } from 'vue-router'
import { startAuthentication } from '@simplewebauthn/browser'
import ICP from '@/components/ICP/ICP.vue'
import SetLanguage from '@/components/SetLanguage/SetLanguage.vue'
import SwitchAppearance from '@/components/SwitchAppearance/SwitchAppearance.vue'
import Authorization from '@/components/TwoFA/Authorization.vue'
import logo from '@/assets/img/logo-primadigi.png'
import background from '@/assets/img/login.mp4'

const thisYear = new Date().getFullYear()
const route = useRoute()
const router = useRouter()

// Existing reactive state
const loading = ref(false)
const enabled2FA = ref(false)
const refOTP = ref(null)
const passcode = ref('')
const recoveryCode = ref('')
const passkeyConfigStatus = ref(false)

const modelRef = reactive({
  username: '',
  password: '',
  captcha: ''
})

const rulesRef = reactive({
  username: [
    {
      required: true,
      message: () => $gettext('Please input your username!')
    }
  ],
  password: [
    {
      required: true,
      message: () => $gettext('Please input your password!')
    }
  ]
})

const { validate, validateInfos } = Form.useForm(modelRef, rulesRef)
const userStore = useUserStore()
const { login, passkeyLogin } = userStore

// Existing submit handler
const onSubmit = () => {
  validate().then(async () => {
    loading.value = true
    try {
      const r = await auth.login(modelRef.username, modelRef.password, passcode.value, recoveryCode.value)
      const next = (route.query?.next || '').toString() || '/'
      
      if (r.code === 200) {
        login(r.token)
        await router.push(next)
      } else if (r.code === 199) {
        enabled2FA.value = true
      }
    } catch (e) {
      if (e.code === 4043) {
        refOTP.value?.clearInput()
      }
    } finally {
      loading.value = false
    }
  })
}
</script>

<template>
  <div class="min-h-screen flex">
    <!-- Left Section -->
    <div class="hidden lg:flex lg:w-2/3 login-bg relative p-12 flex-col justify-center">
      <!-- <div class="absolute inset-0 overflow-hidden">
        <div class="absolute top-0 right-0 w-full h-full opacity-10 bg-dot-pattern"></div>
      </div> -->
      <video 
        class="absolute inset-0 w-full h-full object-cover z-0" 
        autoplay 
        loop 
        muted 
        playsinline
      >
        <source :src="background" type="video/mp4">
      </video>
      <div class="absolute inset-0 bg-black bg-opacity-40 z-1"></div>
      <div class="relative z-10">
        <h1 class="text-4xl font-bold text-white mb-4">PrimeWaf</h1>
        <h2 class="text-2xl text-white mb-8">Simple, Effective, Visible Security</h2>
        <div class="space-y-4 text-gray-300">
          <p>Detect new threats using integrated proactive protection and AI technology.</p>
          <p>Visualize protection with automatic asset discovery throughout every stage of an attack.</p>
          <p>Respond to threats quickly with easy-to-use built-in tools and open APIs.</p>
        </div>
      </div>
    </div>

    <!-- Right Section -->
    <div class="w-full lg:w-1/3 bg-white p-8 flex flex-col">
      <div class="flex justify-between items-center mb-12">
        <div class="flex items-center">
          <img :src="logo" alt="Primadigi Systems" class="h-8 w-8 mr-2" />
          <span class="text-gray-700 font-semibold">PRIMADIGI SYSTEMS</span>
        </div>
        <div class="flex items-center gap-4">
          <SetLanguage class="inline" />
          <SwitchAppearance />
        </div>
      </div>

      <div class="flex-grow flex flex-col items-center justify-center max-w-md mx-auto w-full">
        <div class="mb-8 text-center">
          <img :src="logo" alt="PrimeWaf" class="h-16 w-16 mx-auto mb-2" />
          <h3 class="text-xl font-semibold">PrimeWaf</h3>
          <div class="flex items-center justify-center gap-2 text-sm text-gray-500 mt-1">
            <span>Version: PrimeWaf 8.0.36</span>
            <span class="px-2 py-0.5 bg-blue-100 text-blue-600 rounded">IPv6</span>
          </div>
        </div>

        <a-form v-if="!enabled2FA" class="w-full" @submit.prevent="onSubmit">
          <a-form-item v-bind="validateInfos.username">
            <a-input
              v-model:value="modelRef.username"
              :placeholder="$gettext('Username')"
              size="large"
            >
              <template #prefix>
                <UserOutlined style="color: rgba(0, 0, 0, 0.25)" />
              </template>
            </a-input>
          </a-form-item>

          <a-form-item v-bind="validateInfos.password">
            <a-input-password
              v-model:value="modelRef.password"
              :placeholder="$gettext('Password')"
              size="large"
            >
              <template #prefix>
                <LockOutlined style="color: rgba(0, 0, 0, 0.25)" />
              </template>
            </a-input-password>
          </a-form-item>

          <a-button
            type="primary"
            block
            size="large"
            :loading="loading"
            html-type="submit"
          >
            {{ $gettext('Log In') }}
          </a-button>
        </a-form>

        <Authorization
          v-else
          ref="refOTP"
          :two-f-a-status="{
            enabled: true,
            otp_status: true,
            passkey_status: false
          }"
          @submit-o-t-p="handleOTPSubmit"
        />
      </div>

      <div class="text-center text-sm text-gray-500 mt-8">
        <p class="mb-4">Copyright © 2011-{{ thisYear }} Primadigi Systems International. All rights reserved</p>
        <ICP />
      </div>
    </div>
  </div>
</template>

<style lang="less" scoped>
.bg-dot-pattern {
  background-image: radial-gradient(circle, rgba(255,255,255,0.1) 1px, transparent 1px);
  background-size: 20px 20px;
}

.login-bg {
  position: relative;
  overflow: hidden;
}

:deep(.ant-input-affix-wrapper) {
  border-radius: 4px;
}

:deep(.ant-btn-primary) {
  background-color: #1677ff;
  
  &:hover {
    background-color: #4096ff;
  }
}

.dark {
  .ant-layout-content {
    background: transparent;
  }
  
  .bg-white {
    background-color: #1f1f1f;
  }
  
  .text-gray-700 {
    color: #e5e5e5;
  }
}
</style>