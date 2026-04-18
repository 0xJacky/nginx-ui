<script setup lang="ts">
import settings from '@/api/settings'
import { use2FAModal } from '@/components/TwoFA'

const props = defineProps<{
  value: string
  path: string
}>()

const { message } = useGlobalApp()
const twoFAModal = use2FAModal()
const show = ref(false)
const isLoading = ref(false)
const revealedValue = ref('')

function maskText(text: string) {
  if (!text)
    return '*********'

  if (text.length <= 10)
    return '*********'

  const start = text.substring(0, Math.floor((text.length - 10) / 2))
  const end = text.substring(start.length + 10)

  return `${start}**********${end}`
}

async function ensureRevealedValue() {
  if (revealedValue.value)
    return revealedValue.value

  isLoading.value = true
  try {
    await twoFAModal.open()
    const { value } = await settings.get_protected_value(props.path)
    revealedValue.value = value
    return value
  }
  finally {
    isLoading.value = false
  }
}

const maskedString = computed(() => {
  if (show.value)
    return revealedValue.value

  return maskText(revealedValue.value || props.value)
})

async function toggleShow() {
  if (!show.value)
    await ensureRevealedValue()
  show.value = !show.value
}

async function copyValue() {
  const value = await ensureRevealedValue()
  await navigator.clipboard.writeText(value)
  message.success($gettext('Copied'))
}
</script>

<template>
  <div>
    <span class="mr-2">{{ maskedString }}</span>
    <a
      class="mr-2"
      @click="copyValue"
    >
      {{ $gettext('Copy') }}
    </a>
    <a @click="toggleShow">{{ show ? $gettext('Hide') : isLoading ? $gettext('Loading...') : $gettext('Show') }}</a>
  </div>
</template>

<style scoped lang="less">

</style>
