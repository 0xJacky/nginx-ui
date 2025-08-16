<script setup lang="ts">
import { message, Modal } from 'ant-design-vue'
import nginxLog from '@/api/nginx_log'

// Props
interface Props {
  disabled?: boolean
}

const props = withDefaults(defineProps<Props>(), {
  disabled: false,
})

// Emits
const emit = defineEmits<{
  refresh: []
}>()

// Reactive state
const loading = ref(false)

// Rebuild entire index
async function rebuildIndex() {
  Modal.confirm({
    title: $gettext('Rebuild Index'),
    content: $gettext('This will rebuild the entire log index. All existing index data will be deleted and rebuilt from scratch. This may take some time. Continue?'),
    okText: $gettext('Yes'),
    okType: 'danger',
    cancelText: $gettext('Cancel'),
    async onOk() {
      try {
        loading.value = true
        const response = await nginxLog.rebuildIndex()
        message.success(response.message)
        await nextTick()
        emit('refresh')
      }
      catch (error: unknown) {
        console.error(error)
      }
      finally {
        loading.value = false
      }
    },
  })
}

// Rebuild specific file index
async function rebuildFileIndex(path: string) {
  Modal.confirm({
    title: $gettext('Rebuild File Index'),
    content: $gettext('This will rebuild the index data for this specific file: {path}', { path }),
    okText: $gettext('Yes'),
    okType: 'primary',
    cancelText: $gettext('Cancel'),
    async onOk() {
      try {
        loading.value = true
        const response = await nginxLog.rebuildFileIndex(path)
        message.success(response.message)
        // 确保刷新在成功后立即执行
        await nextTick()
        emit('refresh')
      }
      catch (error) {
        console.error(error)
      }
      finally {
        loading.value = false
      }
    },
  })
}

// Expose rebuildFileIndex for parent component
defineExpose({
  rebuildFileIndex,
})
</script>

<template>
  <div class="index-management">
    <AButton
      type="link"
      size="small"
      :loading="loading"
      :disabled="props.disabled"
      @click="rebuildIndex"
    >
      {{ $gettext('Rebuild All Index') }}
    </AButton>
  </div>
</template>

<style scoped lang="less">

</style>
