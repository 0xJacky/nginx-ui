<script setup lang="ts">
import type { TwoFAStatus } from '@/api/2fa'
import { KeyOutlined } from '@ant-design/icons-vue'
import { startAuthentication } from '@simplewebauthn/browser'
import twoFA from '@/api/2fa'
import OTPInput from '@/components/OTPInput'
import { useUserStore } from '@/pinia'

defineProps<{
  twoFAStatus: TwoFAStatus
}>()

const emit = defineEmits(['submitOTP', 'submitSecureSessionID'])

const user = useUserStore()
const refOTP = useTemplateRef('refOTP')
const useRecoveryCode = ref(false)
const passcode = ref('')
const recoveryCode = ref('')
const passkeyLoading = ref(false)

function clickUseRecoveryCode() {
  passcode.value = ''
  useRecoveryCode.value = true
}

function clickUseOTP() {
  passcode.value = ''
  useRecoveryCode.value = false
}

function onSubmit() {
  emit('submitOTP', passcode.value, recoveryCode.value)
}

function clearInput() {
  refOTP.value?.clearInput()
}

defineExpose({
  clearInput,
})

async function passkeyAuthenticate() {
  passkeyLoading.value = true

  const begin = await twoFA.begin_start_secure_session_by_passkey()
  const asseResp = await startAuthentication({ optionsJSON: begin.options.publicKey })

  const r = await twoFA.finish_start_secure_session_by_passkey({
    session_id: begin.session_id,
    options: asseResp,
  })

  emit('submitSecureSessionID', r.session_id)

  passkeyLoading.value = false
}

onMounted(() => {
  if (user.passkeyLoginAvailable)
    passkeyAuthenticate()
})
</script>

<template>
  <div>
    <div
      v-if="useRecoveryCode"
      class="mt-2 mb-4"
    >
      <p>{{ $gettext('Input the recovery code:') }}</p>
      <AInputGroup compact>
        <AInput v-model:value="recoveryCode" placeholder="xxxxx-xxxxx" />
        <AButton
          type="primary"
          @click="onSubmit"
        >
          {{ $gettext('Recovery') }}
        </AButton>
      </AInputGroup>
    </div>

    <div v-if="twoFAStatus.otp_status && !useRecoveryCode">
      <p>{{ $gettext('Please enter the OTP code:') }}</p>
      <OTPInput
        ref="refOTP"
        v-model="passcode"
        class="justify-center mb-6"
        @on-complete="onSubmit"
      />
    </div>

    <div
      v-if="twoFAStatus.passkey_status"
      class="flex flex-col justify-center"
    >
      <ADivider v-if="twoFAStatus.otp_status">
        <div class="text-sm font-normal opacity-75">
          {{ $gettext('Or') }}
        </div>
      </ADivider>

      <AButton
        :loading="passkeyLoading"
        @click="passkeyAuthenticate"
      >
        <KeyOutlined />
        {{ $gettext('Authenticate with a passkey') }}
      </AButton>
    </div>

    <div v-if="twoFAStatus.otp_status || twoFAStatus.recovery_codes_generated" class="flex justify-center mt-3">
      <a
        v-if="!useRecoveryCode"
        @click="clickUseRecoveryCode"
      >{{ $gettext('Use recovery code') }}</a>
      <a
        v-else-if="twoFAStatus.otp_status"
        @click="clickUseOTP"
      >{{ $gettext('Use OTP') }}</a>
    </div>
  </div>
</template>

<style scoped lang="less">
:deep(.ant-input-group.ant-input-group-compact) {
  display: flex;
}
</style>
