<script setup lang="ts">
import type { Cert } from '@/api/cert'
import { StdTable } from '@uozi-admin/curd'
import cert from '@/api/cert'
import certColumns from '@/views/certificate/CertificateList/certColumns'

interface Props {
  selectionType?: 'radio' | 'checkbox'
}

const props = withDefaults(defineProps<Props>(), {
  selectionType: 'checkbox',
})

const emit = defineEmits(['change'])

const visible = ref(false)

function open() {
  visible.value = true
}

const records = ref<Cert[]>([])
const selectedKeys = ref<(string | number)[]>([])

// Handle selection changes
function handleSelectionChange(keys: (string | number)[] | string | number) {
  // Ensure selectedKeys is always an array
  if (props.selectionType === 'radio' && !Array.isArray(keys)) {
    selectedKeys.value = keys !== undefined && keys !== null ? [keys] : []
  }
  else if (Array.isArray(keys)) {
    selectedKeys.value = keys
  }
}

async function ok() {
  visible.value = false
  emit('change', records.value)

  records.value = []
  selectedKeys.value = []
}

const columns = computed(() => certColumns.filter(item => item.pure))
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
      <StdTable
        v-model:selected-rows="records"
        :selected-row-keys="selectedKeys"
        :get-list-api="cert.getList"
        only-query
        disable-router-query
        :columns
        :row-selection-type="selectionType"
        :table-props="{
          rowKey: 'id',
        }"
        @update:selected-row-keys="handleSelectionChange"
      />
    </AModal>
  </div>
</template>

<style lang="less" scoped>

</style>
