<script setup lang="ts">
import type { RecoveryCode } from '@/api/recovery'
import { CheckCircleOutlined } from '@antdv-next/icons'
import otp from '@/api/otp'
import { use2FAModal } from '@/components/TwoFA'
import TOTPEnrollment from '@/components/TwoFA/TOTPEnrollment.vue'
import { useUserStore } from '@/pinia'

const { status = false } = defineProps<{ status?: boolean }>()
const emit = defineEmits<{ refresh: [void] }>()
const { message } = App.useApp()
const user = useUserStore()
const recoveryCodes = defineModel<RecoveryCode[]>('recoveryCodes')
const enrolling = ref(false)
const replacing = ref(false)
const loading = ref(false)
const generatedUrl = ref('')
const secret = ref('')
const enrollment = useTemplateRef('enrollment')
const otpModal = use2FAModal()
const cannotDisable = computed(() => user.twoFAStatus.required && !user.twoFAStatus.passkey_status)

async function beginEnrollment() {
  if (status && !await otpModal.open())
    return
  loading.value = true
  try {
    const response = await otp.generate_secret()
    secret.value = response.secret
    generatedUrl.value = response.url
    replacing.value = status
    enrolling.value = true
  }
  finally {
    loading.value = false
  }
}

async function enroll(code: string, password: string) {
  if (!password) {
    message.error($gettext('Please enter your current password'))
    enrollment.value?.clearInput()
    return
  }
  loading.value = true
  try {
    const response = await otp.enroll_otp(secret.value, code, password, replacing.value)
    recoveryCodes.value = response.codes
    enrolling.value = false
    secret.value = ''
    generatedUrl.value = ''
    emit('refresh')
    message.success($gettext('Enable 2FA successfully'))
  }
  catch {
    enrollment.value?.clearInput()
  }
  finally {
    loading.value = false
  }
}

async function disableTOTP() {
  if (!await otpModal.open())
    return
  loading.value = true
  try {
    await otp.reset()
    recoveryCodes.value = undefined
    emit('refresh')
    message.success($gettext('TOTP disabled'))
  }
  finally {
    loading.value = false
  }
}
</script>

<template>
  <div>
    <h3>{{ $gettext('TOTP') }}</h3>
    <p>{{ $gettext('TOTP is a two-factor authentication method that uses a time-based one-time password algorithm.') }}</p>
    <p>{{ $gettext('To enable it, you need to install the Google or Microsoft Authenticator app on your mobile phone.') }}</p>
    <AAlert v-if="!status" type="warning" :title="$gettext('Current account is not enabled TOTP.')" class="mb-2" show-icon />
    <p v-else>
      <CheckCircleOutlined class="mr-2 text-green-600" />{{ $gettext('Current account is enabled TOTP.') }}
    </p>
    <AFlex v-if="!enrolling" wrap gap="small">
      <AButton type="primary" ghost :loading @click="beginEnrollment">
        {{ status ? $gettext('Replace TOTP') : $gettext('Enable TOTP') }}
      </AButton>
      <APopconfirm v-if="status" :disabled="cannotDisable" :title="$gettext('Disable TOTP for this account?')" @confirm="disableTOTP">
        <AButton danger :loading :disabled="cannotDisable">
          {{ $gettext('Disable TOTP') }}
        </AButton>
      </APopconfirm>
    </AFlex>
    <template v-else>
      <AAlert v-if="replacing" class="my-3" type="info" show-icon :title="$gettext('Your current authenticator remains active until the new code is verified. New recovery codes replace the old ones.')" />
      <TOTPEnrollment ref="enrollment" class="mt-4" :secret :url="generatedUrl" :loading require-password @complete="enroll" />
      <AButton class="mt-3" :disabled="loading" @click="enrolling = false; secret = ''; generatedUrl = ''">
        {{ $gettext('Cancel') }}
      </AButton>
    </template>
  </div>
</template>
