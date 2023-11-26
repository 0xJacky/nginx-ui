<script setup lang="ts">
defineProps<{
  prefixCls?: string
  extra?: string | object
}>()
</script>

<template>
  <teleport to="body">
    <div class="ant-pro-footer-toolbar" ref="refToolBar">
      <div style="float: left">
        <slot name="extra">{{ extra }}</slot>
      </div>
      <div style="float: right">
        <slot></slot>
      </div>
    </div>
  </teleport>
</template>

<style lang="less" scoped>
.dark {
  .ant-pro-footer-toolbar {
    background: rgba(24, 24, 24, 1);
    border-top: unset;
  }
}

.ant-pro-footer-toolbar {
  position: sticky;
  width: 100%;
  height: 56px;
  bottom: 0;
  right: 0;
  line-height: 56px;
  box-shadow: 0 -1px 2px rgba(0, 0, 0, 0.03);
  background: #ffffff8c;
  border-top: 1px solid #e8e8e8;
  padding: 0 24px;
  z-index: 9;

  &:after {
    content: "";
    display: block;
    clear: both;
  }
}
</style>
