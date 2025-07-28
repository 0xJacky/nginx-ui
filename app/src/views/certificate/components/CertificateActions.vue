<script setup lang="ts">
import FooterToolBar from '@/components/FooterToolbar'

const emit = defineEmits<{
  save: []
  back: []
}>()

function handleSave() {
  emit('save')
}

function handleBack() {
  emit('back')
}
</script>

<template>
  <FooterToolBar>
    <ASpace>
      <AButton @click="handleBack">
        {{ $gettext('Back') }}
      </AButton>

      <AButton
        type="primary"
        @click="handleSave"
      >
        {{ $gettext('Save') }}
      </AButton>
    </ASpace>
  </FooterToolBar>
</template>

<style scoped lang="less">
</style>
