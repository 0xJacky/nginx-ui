<script setup lang="ts">
import SetLanguage from '@/components/SetLanguage'
import SwitchAppearance from '@/components/SwitchAppearance'

const thisYear = new Date().getFullYear()
</script>

<template>
  <div class="footer">
    <p>Copyright © 2021 - {{ thisYear }} Nginx UI</p>
    Language
    <SetLanguage class="inline" />
    <div class="flex justify-center mt-4">
      <SwitchAppearance />
    </div>
  </div>
</template>

<style lang="less" scoped>
.footer {
  padding: 30px;
  text-align: center;
  font-size: 14px;
}
</style>
