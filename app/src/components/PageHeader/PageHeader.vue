<script setup lang="ts">
import Breadcrumb from '@/components/Breadcrumb/Breadcrumb.vue'

const route = useRoute()

const display = computed(() => {
  return !route.meta.hiddenHeaderContent
})
</script>

<template>
  <div
    v-if="display"
    class="page-header"
  >
    <div class="page-header-index-wide">
      <Breadcrumb />
      <div class="action" />
    </div>
  </div>
</template>

<style lang="less" scoped>
.dark {
  .page-header {
    background: #090909 !important;
    border-bottom: unset;

    h1 {
      color: #fafafa;
    }
  }
}

.page-header {
  background: #fff;
  padding: 16px 32px;
  border-bottom: 1px solid #e8e8e8;

  .page-header-index-wide {
    display: flex;
    align-items: center;
    justify-content: space-between;
    gap: 12px;
  }

  .breadcrumb {
    margin-bottom: 0;
  }

  .action {
    display: inline-flex;
    align-items: center;
    justify-content: flex-end;
    flex: 0 0 auto;
  }
}
</style>
