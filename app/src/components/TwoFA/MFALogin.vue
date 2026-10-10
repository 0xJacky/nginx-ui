<script setup lang="ts">
import type { PublicKeyCredentialCreationOptionsJSON, PublicKeyCredentialRequestOptionsJSON } from '@simplewebauthn/browser'
import type { MFAAuthResponse, MFAPreAuthStatus } from '@/api/mfa'
import { startAuthentication, startRegistration } from '@simplewebauthn/browser'
import { UseClipboard } from '@vueuse/components'
import { useNow } from '@vueuse/core'
import { useMFAPreAuth } from '@/api/mfa'
import OTPInput from '@/components/OTPInput'
import { handleApiError, useMessageDedupe } from '@/lib/http/error'
import { normalizeHttpError } from '@/lib/http/normalizeError'
import TOTPEnrollment from './TOTPEnrollment.vue'

const props = defineProps<{ preAuthId: string }>()
const emit = defineEmits<{
  completed: [response: MFAAuthResponse]
  cancel: []
}>()
const api = useMFAPreAuth(props.preAuthId)
const { message } = useGlobalApp()
const errorMessages = useMessageDedupe()
const status = ref<MFAPreAuthStatus>()
const loading = ref(false)
const hasExpired = ref(false)
const secret = ref('')
const url = ref('')
const passcode = ref('')
const recoveryCode = ref('')
const useRecovery = ref(false)
const result = ref<MFAAuthResponse>()
const enrollment = useTemplateRef('enrollment')
const input = useTemplateRef('input')
const codes = computed(() => result.value?.recovery_codes?.codes.map(code => code.code).join('\n') ?? '')
const now = useNow()
const isExpired = computed(() => hasExpired.value || (!!status.value && now.value.getTime() >= status.value.expires_at * 1000))

async function run(action: () => Promise<void>) {
  if (loading.value)
    return
  loading.value = true
  try {
    await action()
  }
  catch (error) {
    await handleApiError(normalizeHttpError(error), errorMessages)
    enrollment.value?.clearInput()
    input.value?.clearInput()
  }
  finally {
    loading.value = false
  }
}

function complete(response: MFAAuthResponse) {
  if (response.recovery_codes?.codes.length)
    result.value = response
  else
    emit('completed', response)
}

function enrollTOTP(code: string) {
  void run(async () => complete(await api.finishTOTP(code)))
}

function verifyOTP() {
  void run(async () => complete(await api.verifyOTP(useRecovery.value ? '' : passcode.value, useRecovery.value ? recoveryCode.value : '')))
}

function beginTOTP() {
  void run(async () => {
    const response = await api.beginTOTP()
    secret.value = response.secret
    url.value = response.url
  })
}

function authenticatePasskey() {
  void run(async () => {
    const options = await api.beginPasskey()
    const response = status.value?.mfa_stage === 'setup'
      ? await startRegistration({ optionsJSON: options as PublicKeyCredentialCreationOptionsJSON })
      : await startAuthentication({ optionsJSON: options as PublicKeyCredentialRequestOptionsJSON })
    complete(await api.finishPasskey(response))
  })
}

function downloadCodes() {
  const link = document.createElement('a')
  const objectUrl = URL.createObjectURL(new Blob([codes.value], { type: 'text/plain' }))
  link.href = objectUrl
  link.download = 'nginx-ui-recovery-codes.txt'
  link.click()
  URL.revokeObjectURL(objectUrl)
  message.success($gettext('Recovery codes downloaded'))
}

onMounted(() => {
  void run(async () => {
    try {
      status.value = await api.status()
    }
    catch {
      hasExpired.value = true
    }
  })
})
</script>

<template>
  <AFlex vertical gap="middle">
    <template v-if="result">
      <AAlert type="success" show-icon :title="$gettext('MFA setup completed. Save your recovery codes before continuing.')" />
      <AInputTextArea :value="codes" readonly :rows="8" />
      <AFlex wrap gap="small">
        <UseClipboard v-slot="{ copy, copied }">
          <AButton @click="copy(codes)">
            {{ copied ? $gettext('Copied') : $gettext('Copy') }}
          </AButton>
        </UseClipboard>
        <AButton @click="downloadCodes">
          {{ $gettext('Download') }}
        </AButton>
      </AFlex>
      <AButton type="primary" @click="emit('completed', result)">
        {{ $gettext('Continue') }}
      </AButton>
    </template>
    <AAlert v-else-if="isExpired" type="warning" show-icon :title="$gettext('MFA session expired. Please sign in again.')" />
    <template v-else-if="status">
      <AAlert
        type="info"
        show-icon
        :title="status.mfa_stage === 'setup' ? $gettext('MFA is required. Set up an authenticator or a passkey to continue.') : $gettext('Verify your MFA method to continue signing in.')"
      />
      <template v-if="status.mfa_stage === 'setup'">
        <TOTPEnrollment v-if="secret" ref="enrollment" :secret :url :loading @complete="enrollTOTP" />
        <AButton v-else :loading @click="beginTOTP">
          {{ $gettext('Enable TOTP') }}
        </AButton>
      </template>
      <template v-else>
        <AInput v-if="useRecovery" v-model:value="recoveryCode" :placeholder="$gettext('Input the recovery code:')" @press-enter="verifyOTP" />
        <OTPInput v-else-if="status.otp_status" ref="input" v-model="passcode" @on-complete="verifyOTP" />
        <AButton v-if="useRecovery" :loading @click="verifyOTP">
          {{ $gettext('Verify') }}
        </AButton>
        <AButton v-if="status.recovery_codes_generated || status.otp_status" type="link" @click="useRecovery = !useRecovery">
          {{ useRecovery ? $gettext('Use OTP') : $gettext('Use recovery code') }}
        </AButton>
      </template>
      <AButton v-if="status.mfa_stage === 'setup' ? status.passkey_available : status.passkey_status" :loading @click="authenticatePasskey">
        {{ status.mfa_stage === 'setup' ? $gettext('Add a passkey') : $gettext('Authenticate with a passkey') }}
      </AButton>
    </template>
    <ASpin v-else />
    <AButton v-if="!result" type="link" @click="emit('cancel')">
      {{ $gettext('Back to sign in') }}
    </AButton>
  </AFlex>
</template>
