<script setup lang="ts">
defineProps<{
  generate?: boolean
  placeholder?: string
}>()

const modelValue = defineModel<string>('value', {
  default: () => {
    return ''
  },
})

const visibility = ref(false)

function handleGenerate() {
  visibility.value = true
  modelValue.value = 'xxxx'

  const chars = '0123456789abcdefghijklmnopqrstuvwxyz!@#$%^&*()ABCDEFGHIJKLMNOPQRSTUVWXYZ'
  const passwordLength = 12
  let password = ''
  for (let i = 0; i <= passwordLength; i++) {
    // eslint-disable-next-line sonarjs/pseudo-random
    const randomNumber = Math.floor(Math.random() * chars.length)

    password += chars.substring(randomNumber, randomNumber + 1)
  }

  modelValue.value = password
}
</script>

<template>
  <div>
    <AInputGroup compact>
      <AInputPassword
        v-if="!visibility"
        v-model:value="modelValue"
        :class="{ compact: generate }"
        :placeholoder="placeholder"
        :maxlength="20"
      />
      <AInput
        v-else
        v-model:value="modelValue"
        :class="{ compact: generate }"
        :placeholoder="placeholder"
        :maxlength="20"
      />
      <AButton
        v-if="generate"
        type="primary"
        @click="handleGenerate"
      >
        {{ $gettext('Generate') }}
      </AButton>
    </AInputGroup>
  </div>
</template>

<style lang="less" scoped>
:deep(.ant-input-group.ant-input-group-compact) {
  display: flex;
}
</style>
