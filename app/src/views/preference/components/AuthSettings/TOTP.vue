<script setup lang="ts">
import type { RecoveryCode } from '@/api/recovery'
import { CheckCircleOutlined } from '@ant-design/icons-vue'
import { UseClipboard } from '@vueuse/components'
import otp from '@/api/otp'
import OTPInput from '@/components/OTPInput'
import { use2FAModal } from '@/components/TwoFA'

const { status = false } = defineProps<{
  status?: boolean
}>()

const emit = defineEmits<{
  refresh: [void]
}>()

const { message } = App.useApp()

const recoveryCodes = defineModel<RecoveryCode[]>('recoveryCodes')

const enrolling = ref(false)
const resetting = ref(false)
const generatedUrl = ref('')
const secret = ref('')
const passcode = ref('')
const refOtp = useTemplateRef('refOtp')

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
    recoveryCodes.value = r.codes
    emit('refresh')
    message.success($gettext('Enable 2FA successfully'))
  }).catch(() => {
    refOtp.value?.clearInput()
  })
}

function reset2FA() {
  const otpModal = use2FAModal()
  otpModal.open().then(() => {
    otp.reset().then(() => {
      resetting.value = false
      recoveryCodes.value = undefined
      emit('refresh')
      clickEnable2FA()
    })
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

    <AButton
      v-if="!status && !enrolling"
      type="primary"
      ghost
      @click="clickEnable2FA"
    >
      {{ $gettext('Enable TOTP') }}
    </AButton>
    <APopconfirm
      v-if="status && !resetting"
      :title="$gettext('Are you sure to reset 2FA?')"
      @confirm="reset2FA"
    >
      <AButton
        v-if="status && !resetting"
        type="primary"
        ghost
      >
        {{ $gettext('Reset 2FA') }}
      </AButton>
    </APopconfirm>

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
  </div>
</template>

<style scoped lang="less">
:deep(.ant-input-group.ant-input-group-compact) {
  display: flex;
}
</style>
