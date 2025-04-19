<script setup lang="ts">
import RightSettings from '@/views/site/site_edit/components/RightPanel/RightPanel.vue'
import SiteEditor from '@/views/site/site_edit/components/SiteEditor'
</script>

<template>
  <div class="site-container">
    <ARow :gutter="{ xs: 0, sm: 16 }">
      <ACol
        :xs="24"
        :sm="24"
        :md="24"
        :lg="16"
        :xl="17"
      >
        <div>
          <SiteEditor />
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
  </div>
</template>

<style lang="less" scoped>
.site-container {
  max-height: calc(100vh - 267px);
  overflow-y: auto;
}

:deep(.ant-card) {
  box-shadow: unset;
}
</style>
