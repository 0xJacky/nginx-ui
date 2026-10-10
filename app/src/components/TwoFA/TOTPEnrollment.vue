<script setup lang="ts">
import { UseClipboard } from '@vueuse/components'
import OTPInput from '@/components/OTPInput'

defineProps<{
  secret: string
  url: string
  requirePassword?: boolean
  loading?: boolean
}>()

const emit = defineEmits<{
  complete: [code: string, password: string]
}>()
const password = ref('')
const passcode = ref('')
const input = useTemplateRef('input')

function clearInput() {
  input.value?.clearInput()
}

defineExpose({ clearInput })
</script>

<template>
  <AFlex vertical align="center" gap="middle">
    <AQrcode v-if="url" :value="url" :size="220" />
    <UseClipboard v-slot="{ copy, copied }">
      <AButton type="link" @click="copy(secret)">
        {{ copied ? $gettext('Secret has been copied') : $gettext('Click to copy') }}
      </AButton>
      <code class="break-all">{{ secret }}</code>
    </UseClipboard>
    <AForm v-if="requirePassword" layout="vertical" class="w-full">
      <AFormItem :label="$gettext('Current Password')">
        <AInputPassword v-model:value="password" autocomplete="current-password" />
      </AFormItem>
    </AForm>
    <p>{{ $gettext('Scan the QR code with your mobile phone to add the account to the app.') }}</p>
    <OTPInput
      ref="input"
      v-model="passcode"
      :class="{ 'pointer-events-none opacity-50': loading }"
      @on-complete="code => emit('complete', code, password)"
    />
  </AFlex>
</template>
