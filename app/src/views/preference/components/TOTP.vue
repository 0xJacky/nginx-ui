<script setup lang="ts">
import { message } from 'ant-design-vue'
import { CheckCircleOutlined } from '@ant-design/icons-vue'
import otp from '@/api/otp'
import OTPInput from '@/components/OTPInput/OTPInput.vue'

const status = ref(false)
const enrolling = ref(false)
const resetting = ref(false)
const qrCode = ref('')
const secret = ref('')
const passcode = ref('')
const interval = ref()
const refOtp = ref()
const recoveryCode = ref('')
const inputRecoveryCode = ref('')

function clickEnable2FA() {
  enrolling.value = true
  generateSecret()
  interval.value = setInterval(() => {
    if (enrolling.value)
      generateSecret()
    else
      clearGenerateSecretInterval()
  }, 30 * 1000)
}

function clearGenerateSecretInterval() {
  if (interval.value) {
    clearInterval(interval.value)
    interval.value = undefined
  }
}

function generateSecret() {
  otp.generate_secret().then(r => {
    secret.value = r.secret
    qrCode.value = r.qr_code
    refOtp.value?.clearInput()
  }).catch((e: { message?: string }) => {
    message.error(e.message ?? $gettext('Server error'))
  })
}

function enroll(code: string) {
  otp.enroll_otp(secret.value, code).then(r => {
    enrolling.value = false
    recoveryCode.value = r.recovery_code
    clearGenerateSecretInterval()
    get2FAStatus()
    message.success($gettext('Enable 2FA successfully'))
  }).catch((e: { message?: string }) => {
    refOtp.value?.clearInput()
    message.error(e.message ?? $gettext('Server error'))
  })
}

function get2FAStatus() {
  otp.status().then(r => {
    status.value = r.status
  })
}

get2FAStatus()

onUnmounted(clearGenerateSecretInterval)

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
  }).catch((e: { message?: string }) => {
    message.error($gettext(e.message ?? 'Server error'))
  })
}
</script>

<template>
  <div>
    <h3>{{ $gettext('2FA Settings') }}</h3>
    <p>{{ $gettext('TOTP is a two-factor authentication method that uses a time-based one-time password algorithm.') }}</p>
    <p>{{ $gettext('To enable it, you need to install the Google or Microsoft Authenticator app on your mobile phone.') }}</p>
    <p>{{ $gettext('Scan the QR code with your mobile phone to add the account to the app.') }}</p>
    <p v-if="!status">
      {{ $gettext('Current account is not enabled 2FA.') }}
    </p>
    <div v-else>
      <p><CheckCircleOutlined class="mr-2 text-green-600" />{{ $gettext('Current account is enabled 2FA.') }}</p>
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
      {{ $gettext('Enable 2FA') }}
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
      <div class="w-64 h-64 mt-4 mb-2">
        <img
          v-if="qrCode"
          class="w-full"
          :src="qrCode"
          alt="qr code"
        >
      </div>

      <div>
        <p>{{ $gettext('Input the code from the app:') }}</p>
        <OTPInput
          ref="refOtp"
          v-model="passcode"
          @on-complete="enroll"
        />
      </div>
    </template>

    <div
      v-if="resetting"
      class="mt-2"
    >
      <p>{{ $gettext('Input the recovery code:') }}</p>
      <AInputGroup compact>
        <AInput
          v-model:value="inputRecoveryCode"
          style="width: calc(100% - 92px)"
        />
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

</style>
