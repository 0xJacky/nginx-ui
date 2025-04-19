<script setup lang="ts">
import RightSettings from '@/views/stream/components/RightPanel'
import StreamEditor from '@/views/stream/components/StreamEditor.vue'
import { useStreamEditorStore } from '@/views/stream/store'

const route = useRoute()

const name = computed(() => decodeURIComponent(route.params?.name?.toString() ?? ''))

const store = useStreamEditorStore()

onMounted(() => {
  store.init(name.value)
})
</script>

<template>
  <ARow :gutter="{ xs: 0, sm: 16 }">
    <ACol
      :xs="24"
      :sm="24"
      :md="24"
      :lg="16"
      :xl="17"
    >
      <div>
        <StreamEditor />
      </div>
    </ACol>

    <ACol
      class="col-right"
      :xs="24"
      :sm="24"
      :md="24"
      :lg="8"
      :xl="7"
    >
      <RightSettings />
    </ACol>
  </ARow>
</template>

<style lang="less" scoped>
.col-right {
  position: sticky;
  top: 78px;
}

:deep(.ant-card) {
  box-shadow: unset;
}

.slide-fade-enter-active {
  transition: all .3s ease-in-out;
}

.slide-fade-leave-active {
  transition: all .3s cubic-bezier(1.0, 0.5, 0.8, 1.0);
}

.slide-fade-enter-from, .slide-fade-enter-to, .slide-fade-leave-to
  /* .slide-fade-leave-active for below version 2.1.8 */ {
  transform: translateX(10px);
  opacity: 0;
}

.directive-params-wrapper {
  margin: 10px 0;
}

.tab-content {
  padding: 10px;
}
</style>
