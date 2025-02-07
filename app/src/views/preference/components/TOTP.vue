<script setup lang="ts">
import twoFA from '@/api/2fa'
import otp from '@/api/otp'
import OTPInput from '@/components/OTPInput/OTPInput.vue'
import { CheckCircleOutlined } from '@ant-design/icons-vue'
import { UseClipboard } from '@vueuse/components'

import { message } from 'ant-design-vue'

const status = ref(false)
const enrolling = ref(false)
const resetting = ref(false)
const generatedUrl = ref('')
const secret = ref('')
const passcode = ref('')
const refOtp = useTemplateRef('refOtp')
const recoveryCode = ref('')
const inputRecoveryCode = ref('')

function clickEnable2FA() {
  enrolling.value = true
  generateSecret()
}

function generateSecret() {
  otp.generate_secret().then(r => {
    secret.value = r.secret
    generatedUrl.value = r.url
    refOtp.value?.clearInput()
  })
}

function enroll(code: string) {
  otp.enroll_otp(secret.value, code).then(r => {
    enrolling.value = false
    recoveryCode.value = r.recovery_code
    get2FAStatus()
    message.success($gettext('Enable 2FA successfully'))
  }).catch(() => {
    refOtp.value?.clearInput()
  })
}

function get2FAStatus() {
  twoFA.status().then(r => {
    status.value = r.otp_status
  })
}

get2FAStatus()

function clickReset2FA() {
  resetting.value = true
  inputRecoveryCode.value = ''
}

function reset2FA() {
  otp.reset(inputRecoveryCode.value).then(() => {
    resetting.value = false
    recoveryCode.value = ''
    get2FAStatus()
    clickEnable2FA()
  })
}
</script>

<template>
  <div>
    <h3>{{ $gettext('TOTP') }}</h3>
    <p>{{ $gettext('TOTP is a two-factor authentication method that uses a time-based one-time password algorithm.') }}</p>
    <p>{{ $gettext('To enable it, you need to install the Google or Microsoft Authenticator app on your mobile phone.') }}</p>
    <p>{{ $gettext('Scan the QR code with your mobile phone to add the account to the app.') }}</p>
    <AAlert v-if="!status" type="warning" :message="$gettext('Current account is not enabled TOTP.')" class="mb-2" show-icon />
    <div v-else>
      <p><CheckCircleOutlined class="mr-2 text-green-600" />{{ $gettext('Current account is enabled TOTP.') }}</p>
    </div>

    <AAlert
      v-if="recoveryCode"
      :message="$gettext('Recovery Code')"
      class="mb-4"
      type="info"
      show-icon
    >
      <template #description>
        <div>
          <p>{{ $gettext('If you lose your mobile phone, you can use the recovery code to reset your 2FA.') }}</p>
          <p>{{ $gettext('The recovery code is only displayed once, please save it in a safe place.') }}</p>
          <p>{{ $gettext('Recovery Code:') }}</p>
          <span class="ml-2">{{ recoveryCode }}</span>
        </div>
      </template>
    </AAlert>

    <AButton
      v-if="!status && !enrolling"
      type="primary"
      ghost
      @click="clickEnable2FA"
    >
      {{ $gettext('Enable TOTP') }}
    </AButton>
    <AButton
      v-if="status && !resetting"
      type="primary"
      ghost
      @click="clickReset2FA"
    >
      {{ $gettext('Reset 2FA') }}
    </AButton>

    <template v-if="enrolling">
      <div class="flex flex-col items-center">
        <div class="mt-4 mb-2">
          <AQrcode
            v-if="generatedUrl"
            :value="generatedUrl"
            :size="256"
          />
          <div class="w-64 flex justify-center mt-2">
            <UseClipboard v-slot="{ copy, copied }">
              <ATooltip @click="() => copy(secret)">
                <template #title>
                  {{ copied ? $gettext('Secret has been copied')
                    : $gettext('Click to copy') }}
                </template>
                {{ $gettext('Or enter the secret: %{secret}', { secret }) }}
              </ATooltip>
            </UseClipboard>
          </div>
        </div>

        <div>
          <p>{{ $gettext('Input the code from the app:') }}</p>
          <OTPInput
            ref="refOtp"
            v-model="passcode"
            @on-complete="enroll"
          />
        </div>
      </div>
    </template>

    <div
      v-if="resetting"
      class="mt-2"
    >
      <p>{{ $gettext('Input the recovery code:') }}</p>
      <AInputGroup compact>
        <AInput v-model:value="inputRecoveryCode" />
        <AButton
          type="primary"
          @click="reset2FA"
        >
          {{ $gettext('Recovery') }}
        </AButton>
      </AInputGroup>
    </div>
  </div>
</template>

<style scoped lang="less">
:deep(.ant-input-group.ant-input-group-compact) {
  display: flex;
}
</style>
