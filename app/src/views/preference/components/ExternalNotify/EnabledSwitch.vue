<script setup lang="ts">
import type { ExternalNotify } from '@/api/external_notify'
import { message } from 'ant-design-vue'
import externalNotify from '@/api/external_notify'

const props = defineProps<{
  record: ExternalNotify
}>()

const loading = ref(false)
const enabled = defineModel<boolean>('enabled')

async function handleChange(checked) {
  if (!props.record.id)
    return

  loading.value = true
  try {
    await externalNotify.updateItem(props.record.id, {
      enabled: checked,
    })
    // 更新本地状态
    message.success($gettext('Status updated successfully'))
  }
  catch (error) {
    console.error('Update enabled status error:', error)
    // 出错时恢复原状态
    enabled.value = props.record.enabled
    message.error($gettext('Failed to update status'))
  }
  finally {
    loading.value = false
  }
}
</script>

<template>
  <ASwitch
    v-model:checked="enabled"
    :loading="loading"
    size="small"
    @change="handleChange"
  />
</template>
