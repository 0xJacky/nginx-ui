<script setup lang="ts">
import { LoadingOutlined } from '@ant-design/icons-vue'

// Generic editor layout with left and right panels
interface BaseEditorProps {
  colRightClass?: string
  loading?: boolean
}

const props = withDefaults(defineProps<BaseEditorProps>(), {
  colRightClass: 'col-right',
})

const indicator = h(LoadingOutlined, {
  style: {
    fontSize: '32px',
  },
  spin: true,
})

const route = useRoute()
const loading = computed(() =>
  props.loading || (import.meta.env.DEV && route.query.loading === 'true'),
)
</script>

<template>
  <ASpin class="h-full base-editor-spin" :spinning="loading" :indicator="indicator">
    <ARow :gutter="{ xs: 0, sm: 16 }">
      <ACol
        :xs="24"
        :sm="24"
        :md="24"
        :lg="16"
        :xl="17"
      >
        <!-- Left panel content (main editor) -->
        <slot name="left" />
      </ACol>

      <ACol
        :class="props.colRightClass"
        :xs="24"
        :sm="24"
        :md="24"
        :lg="8"
        :xl="7"
      >
        <!-- Right panel content (settings/configuration) -->
        <slot name="right" />
      </ACol>
    </ARow>
  </ASpin>
</template>

<style lang="less" scoped>
.col-right {
  position: sticky;
  top: 78px;
}

:deep(.ant-card) {
  box-shadow: unset;
}

:deep(.card-body) {
  max-height: calc(100vh - 260px);
  overflow-y: scroll;
  padding: 0;
}

:deep(.ant-spin) {
  background: rgba(255, 255, 255, 0.8);
  backdrop-filter: blur(10px);
  -webkit-backdrop-filter: blur(10px);
  max-height: 100% !important;
  border-radius: 8px;
}
</style>

<style lang="less">
.dark {
  .base-editor-spin {
    background: rgba(30, 30, 30, 0.8);
  }
}
</style>
