<script setup lang="ts">
import BaseEditor from '@/components/BaseEditor'
import RightSettings from '@/views/stream/components/RightPanel'
import StreamEditor from '@/views/stream/components/StreamEditor.vue'
import { useStreamEditorStore } from '@/views/stream/store'

const route = useRoute()

const name = computed(() => decodeURIComponent(route.params?.name?.toString() ?? ''))

const store = useStreamEditorStore()
const { loading } = storeToRefs(store)

onMounted(() => {
  store.init(name.value)
})
</script>

<template>
  <BaseEditor :loading>
    <template #left>
      <StreamEditor />
    </template>

    <template #right>
      <RightSettings />
    </template>
  </BaseEditor>
</template>

<style lang="less" scoped>
// Animation styles for mode switching
.slide-fade-enter-active {
  transition: all .3s ease-in-out;
}

.slide-fade-leave-active {
  transition: all .3s cubic-bezier(1.0, 0.5, 0.8, 1.0);
}

.slide-fade-enter-from, .slide-fade-enter-to, .slide-fade-leave-to {
  transform: translateX(10px);
  opacity: 0;
}

// Stream-specific styles
.directive-params-wrapper {
  margin: 10px 0;
}

:deep(.ant-card-body) {
  max-height: 100%;
  overflow-y: scroll;
  padding: 0;
}
</style>
