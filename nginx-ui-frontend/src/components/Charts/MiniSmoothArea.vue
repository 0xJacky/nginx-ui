<template>
  <div :class="prefixCls">
    <div class="chart-wrapper" :style="{ height: 46 }">
      <v-chart :force-fit="true" :height="100" :data="dataSource" :scale="scale" :padding="[36, 0, 18, 0]">
        <v-tooltip />
        <v-smooth-line position="x*y" :size="1" />
        <v-smooth-area position="x*y" />
      </v-chart>
    </div>
  </div>
</template>

<script>
export default {
  name: 'MiniSmoothArea',
  props: {
    prefixCls: {
      type: String,
      default: 'ant-pro-smooth-area'
    },
    scale: {
      type: [Object, Array],
    },
    dataSource: {
      type: Array,
      required: true
    }
  },
  data () {
    return {
      height: 100
    }
  }
}
</script>

<style lang="less" scoped>
  @import "smooth.area.less";
</style>
