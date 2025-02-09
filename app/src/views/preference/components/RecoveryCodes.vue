<script setup lang="ts">
import type { TwoFAStatus } from '@/api/2fa'
import type { RecoveryCode } from '@/api/recovery'
import recovery from '@/api/recovery'
import use2FAModal from '@/components/TwoFA/use2FAModal'
import { CopyOutlined, WarningOutlined } from '@ant-design/icons-vue'
import { UseClipboard } from '@vueuse/components'
import { message } from 'ant-design-vue'

const props = defineProps<{
  recoveryCodes?: RecoveryCode[]
  twoFAStatus?: TwoFAStatus
}>()

const emit = defineEmits<{
  refresh: [void]
}>()

const _codes = ref<RecoveryCode[]>()
const codes = computed(() => props.recoveryCodes ?? _codes.value)
const newGenerated = ref(false)

const codeSource = computed(() => codes.value?.map(code => code.code).join('\n'))

function clickGenerateRecoveryCodes() {
  const otpModal = use2FAModal()
  otpModal.open().then(() => {
    recovery.generate().then(r => {
      _codes.value = r.codes
      newGenerated.value = true
      emit('refresh')
      message.success($gettext('Generate recovery codes successfully'))
    })
  })
}

function clickViewRecoveryCodes() {
  const otpModal = use2FAModal()
  otpModal.open().then(() => {
    recovery.view().then(r => {
      _codes.value = r.codes
    })
  })
}

const popOpen = ref(false)

function popConfirm() {
  popOpen.value = false
  clickGenerateRecoveryCodes()
}

function handlePopOpenChange(visible: boolean) {
  popOpen.value = visible
  if (!visible)
    return

  if (props.twoFAStatus?.recovery_codes_generated)
    popOpen.value = true
  else
    popConfirm()
}
</script>

<template>
  <div>
    <h3>
      {{ $gettext('Recovery Codes') }}
      <ATag v-if="recoveryCodes || twoFAStatus?.recovery_codes_viewed" :color="newGenerated || recoveryCodes ? 'success' : 'processing'">
        {{ newGenerated || recoveryCodes ? $gettext('First View') : $gettext('Viewed') }}
      </ATag>
    </h3>
    <p>{{ $gettext('Recovery codes are used to access your account when you lose access to your 2FA device. Each code can only be used once.') }}</p>
    <p>{{ $gettext('Keep your recovery codes as safe as your password. We recommend saving them with a password manager.') }}</p>

    <AAlert
      v-if="!twoFAStatus?.enabled"
      class="mb-4"
      type="info"
      show-icon
      :message="$gettext('You have not enabled 2FA yet. Please enable 2FA to generate recovery codes.')"
    />
    <AAlert
      v-else-if="!twoFAStatus?.recovery_codes_generated"
      class="mb-4"
      type="warning"
      show-icon
    >
      <template #message>
        <template v-if="twoFAStatus?.otp_status">
          {{ $gettext('Your current recovery code might be outdated and insecure. Please generate new recovery codes at your earliest convenience to ensure security.') }}
        </template>
        <template v-else>
          {{ $gettext('You have not generated recovery codes yet.') }}
        </template>
      </template>
    </AAlert>

    <ACard v-if="codes" class="codes-card mb-4">
      <template #title>
        <AAlert class="whitespace-normal px-6 py-4 rounded-t-[8px]" type="warning" banner :show-icon="false">
          <template #message>
            <WarningOutlined class="ant-alert-icon text-lg" />
            {{ $gettext('These codes are the last resort for accessing your account in case you lose your password and second factors. If you cannot find these codes, you will lose access to your account.') }}
          </template>
        </AAlert>
      </template>
      <ul class="grid grid-cols-2 gap-2 text-lg">
        <li v-for="(code, index) in codes" :key="index">
          <span :class="{ 'line-through': code.used_time }">
            {{ `${code.code.slice(0, 5)}-${code.code.slice(5)}` }}
          </span>
        </li>
      </ul>
      <div class="mt-4 flex space-x-2">
        <UseClipboard v-slot="{ copy, copied }" :source="codeSource">
          <AButton @click="copy()">
            <template #icon>
              <CopyOutlined />
            </template>
            {{ !copied ? $gettext('Copy Codes') : $gettext('Copied') }}
          </AButton>
        </UseClipboard>
      </div>
    </ACard>

    <template v-if="twoFAStatus?.enabled">
      <AButton
        v-if="twoFAStatus?.recovery_codes_generated && !codes"
        type="primary"
        ghost
        @click="clickViewRecoveryCodes"
      >
        {{ $gettext('View Recovery Codes') }}
      </AButton>

      <div v-if="twoFAStatus?.recovery_codes_generated" class="mt-4">
        <h3>{{ $gettext('Generate New Recovery Codes') }}</h3>
        <p>
          {{ $gettext('When you generate new recovery codes, you must download or print the new codes.') }}
          <b>
            {{ $gettext('Your old codes won\'t work anymore.') }}
          </b>
        </p>
      </div>

      <APopconfirm
        :open="popOpen"
        @open-change="handlePopOpenChange"
        @confirm="popConfirm"
        @cancel="() => popOpen = false"
      >
        <template #title>
          {{ $gettext('Are you sure to generate new recovery codes?') }}<br>
          <b>{{ $gettext('Your old codes won\'t work anymore.') }}</b>
        </template>
        <AButton
          type="primary"
          ghost
        >
          {{ twoFAStatus?.recovery_codes_generated ? $gettext('Generate New Recovery Codes') : $gettext('Generate Recovery Codes') }}
        </AButton>
      </APopconfirm>
    </template>
  </div>
</template>

<style scoped lang="less">
.codes-card :deep(.ant-card-head) {
  padding: 0;
}
</style>
