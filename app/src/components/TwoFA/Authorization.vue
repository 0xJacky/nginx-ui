<script setup lang="ts">
import type { TwoFAStatusResponse } from '@/api/2fa'
import twoFA from '@/api/2fa'
import OTPInput from '@/components/OTPInput/OTPInput.vue'
import { useUserStore } from '@/pinia'
import { KeyOutlined } from '@ant-design/icons-vue'
import { startAuthentication } from '@simplewebauthn/browser'
import { message } from 'ant-design-vue'

defineProps<{
  twoFAStatus: TwoFAStatusResponse
}>()

const emit = defineEmits(['submitOTP', 'submitSecureSessionID'])

const user = useUserStore()
const refOTP = ref()
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
  try {
    const begin = await twoFA.begin_start_secure_session_by_passkey()
    const asseResp = await startAuthentication({ optionsJSON: begin.options.publicKey })

    const r = await twoFA.finish_start_secure_session_by_passkey({
      session_id: begin.session_id,
      options: asseResp,
    })

    emit('submitSecureSessionID', r.session_id)
  }
  // eslint-disable-next-line ts/no-explicit-any
  catch (e: any) {
    message.error($gettext(e.message ?? 'Server error'))
  }
  passkeyLoading.value = false
}

onMounted(() => {
  if (user.passkeyLoginAvailable)
    passkeyAuthenticate()
})
</script>

<template>
  <div>
    <div v-if="twoFAStatus.otp_status">
      <div v-if="!useRecoveryCode">
        <p>{{ $gettext('Please enter the OTP code:') }}</p>
        <OTPInput
          ref="refOTP"
          v-model="passcode"
          class="justify-center mb-6"
          @on-complete="onSubmit"
        />
      </div>
      <div
        v-else
        class="mt-2 mb-4"
      >
        <p>{{ $gettext('Input the recovery code:') }}</p>
        <AInputGroup compact>
          <AInput v-model:value="recoveryCode" />
          <AButton
            type="primary"
            @click="onSubmit"
          >
            {{ $gettext('Recovery') }}
          </AButton>
        </AInputGroup>
      </div>

      <div class="flex justify-center">
        <a
          v-if="!useRecoveryCode"
          @click="clickUseRecoveryCode"
        >{{ $gettext('Use recovery code') }}</a>
        <a
          v-else
          @click="clickUseOTP"
        >{{ $gettext('Use OTP') }}</a>
      </div>
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
  </div>
</template>

<style scoped lang="less">
:deep(.ant-input-group.ant-input-group-compact) {
  display: flex;
}
</style>
