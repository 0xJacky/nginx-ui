<script setup lang="ts">
import { UseClipboard } from '@vueuse/components'
import { $gettext } from '../../gettext'

const props = defineProps<{
  value: string
}>()

const show = ref(false)

const maskedString = computed(() => {
  const text = props.value
  if (show.value)
    return text

  if (!text || text.length <= 10)
    return '*********'

  const start = text.substring(0, Math.floor((text.length - 10) / 2))
  const end = text.substring(start.length + 10)

  return `${start}**********${end}`
})

function toggleShow() {
  show.value = !show.value
}
</script>

<template>
  <div>
    <span class="mr-2">{{ maskedString }}</span>
    <UseClipboard v-slot="{ copy, copied }">
      <a
        class="mr-2"
        @click="copy(value)"
      >
        {{ copied ? $gettext('Copied') : $gettext('Copy') }}
      </a>
    </UseClipboard>
    <a @click="toggleShow">{{ show ? $gettext('Hide') : $gettext('Show') }}</a>
  </div>
</template>

<style scoped lang="less">

</style>
