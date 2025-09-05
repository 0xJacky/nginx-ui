<script setup lang="ts">
import { Modal } from 'ant-design-vue'
import nginxLog from '@/api/nginx_log'

// Props
interface Props {
  disabled?: boolean
  indexing?: boolean
}

const props = withDefaults(defineProps<Props>(), {
  disabled: false,
  indexing: false,
})

// Emits
const emit = defineEmits<{
  refresh: []
}>()

const { message } = App.useApp()

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
        await nginxLog.rebuildIndex()
        message.success($gettext('Index and statistics rebuild started successfully'))
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
    content: $gettext('This will rebuild the index data for this specific file: %{path}', { path }),
    okText: $gettext('Yes'),
    okType: 'primary',
    cancelText: $gettext('Cancel'),
    async onOk() {
      try {
        loading.value = true
        await nginxLog.rebuildFileIndex(path)
        message.success($gettext('File index rebuild started successfully for %{path}', { path }))
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
      v-if="!props.indexing"
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
