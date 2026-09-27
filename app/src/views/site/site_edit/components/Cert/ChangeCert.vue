<script setup lang="ts">
import type { Cert } from '@/api/cert'
import CertificatePicker from './CertificatePicker.vue'

interface Props {
  selectionType?: 'radio' | 'checkbox'
}

withDefaults(defineProps<Props>(), {
  selectionType: 'checkbox',
})

const emit = defineEmits<{
  change: [certs: Cert[]]
}>()

const visible = ref(false)

function open() {
  visible.value = true
}

const records = ref<Cert[]>([])

async function ok() {
  visible.value = false
  emit('change', records.value)

  records.value = []
}
</script>

<template>
  <div>
    <AButton @click="open">
      {{ $gettext('Change Certificate') }}
    </AButton>
    <AModal
      v-model:open="visible"
      :title="$gettext('Change Certificate')"
      :mask="false"
      width="800px"
      @ok="ok"
    >
      <CertificatePicker
        v-model:selected-rows="records"
        :selection-type
      />
    </AModal>
  </div>
</template>
