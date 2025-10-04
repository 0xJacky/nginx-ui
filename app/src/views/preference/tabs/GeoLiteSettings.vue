<script setup lang="ts">
import GeoLiteDownload from '@/components/GeoLiteDownload'
</script>

<template>
  <AForm layout="vertical">
    <AFormItem :label="$gettext('GeoLite2 Database')">
      <ATypographyParagraph type="secondary">
        {{ $gettext('The GeoLite2 database provides geographic information for IP addresses. This is used for offline geographic analysis in log analytics.') }}
      </ATypographyParagraph>
      <GeoLiteDownload />
    </AFormItem>
  </AForm>
</template>

<style lang="less" scoped>
</style>
