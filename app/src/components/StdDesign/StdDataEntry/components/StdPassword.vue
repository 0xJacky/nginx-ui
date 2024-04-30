<script setup lang="ts">
import { computed, ref } from 'vue'

const props = defineProps<{
  value: string
  generate?: boolean
  placeholder?: string
}>()

const emit = defineEmits(['update:value'])

const M_value = computed({
  get() {
    return props.value
  },
  set(v) {
    emit('update:value', v)
  },
})

const visibility = ref(false)
function handle_generate() {
  visibility.value = true
  M_value.value = 'xxxx'

  const chars = '0123456789abcdefghijklmnopqrstuvwxyz!@#$%^&*()ABCDEFGHIJKLMNOPQRSTUVWXYZ'
  const passwordLength = 12
  let password = ''
  for (let i = 0; i <= passwordLength; i++) {
    const randomNumber = Math.floor(Math.random() * chars.length)

    password += chars.substring(randomNumber, randomNumber + 1)
  }

  M_value.value = password
}
</script>

<template>
  <AInputGroup compact>
    <AInputPassword
      v-if="!visibility"
      v-model:value="M_value"
      :class="{ compact: generate }"
      :placeholoder="placeholder"
    />
    <AInput
      v-else
      v-model:value="M_value"
      :class="{ compact: generate }"
      :placeholoder="placeholder"
    />
    <AButton
      v-if="generate"
      type="primary"
      @click="handle_generate"
    >
      {{ $gettext('Generate') }}
    </AButton>
  </AInputGroup>
</template>

<style scoped>
.compact {
  width: calc(100% - 91px)
}
</style>
