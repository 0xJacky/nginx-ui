<script setup lang="ts">
import OTPInput from '@/components/OTPInput/OTPInput.vue'

const emit = defineEmits(['onSubmit'])

const refOTP = ref()
const useRecoveryCode = ref(false)
const passcode = ref('')
const recoveryCode = ref('')

function clickUseRecoveryCode() {
  passcode.value = ''
  useRecoveryCode.value = true
}

function clickUseOTP() {
  passcode.value = ''
  useRecoveryCode.value = false
}

function onSubmit() {
  emit('onSubmit', passcode.value, recoveryCode.value)
}

function clearInput() {
  refOTP.value?.clearInput()
}

defineExpose({
  clearInput,
})
</script>

<template>
  <div>
    <div v-if="!useRecoveryCode">
      <p>{{ $gettext('Please enter the 2FA code:') }}</p>
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
</template>

<style scoped lang="less">
:deep(.ant-input-group.ant-input-group-compact) {
  display: flex;
}
</style>
