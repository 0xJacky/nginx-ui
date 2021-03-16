<template>
  <div class="antv-chart-mini">
    <div class="chart-wrapper" :style="{ height: 46 }">
      <v-chart :force-fit="true" :height="height" :data="data" :padding="[36, 5, 18, 5]">
        <v-tooltip/>
        <v-bar position="x*y"/>
      </v-chart>
    </div>
  </div>
</template>

<script>
  const tooltip = [
    'x*y',
    (x, y) => ({
      name: x,
      value: y
    })
  ]

  export default {
    name: 'MiniBar',
    props: {
      data: Array,
    },
    data() {
      return {
        tooltip,
        height: 100
      }
    }
  }
</script>

<style lang="less" scoped>
  @import "chart";
</style>
