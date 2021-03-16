<template>
  <div class="antv-chart-mini">
    <div class="chart-wrapper" :style="{ height: 46 }">
      <v-chart :force-fit="true" :height="height" :data="data" :padding="[36, 0, 18, 0]">
        <v-tooltip/>
        <v-smooth-area position="x*y"/>
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
  const scale = [{
    dataKey: 'x',
    min: 2
  }, {
    dataKey: 'y',
    title: '时间',
    min: 1,
    max: 22
  }]

  export default {
    name: 'MiniArea',
    props: {
      data: {
        type: Array,
        default: null
      },
    },
    data() {
      return {
        dataConfig: this.data,
        tooltip,
        scale,
        height: 100
      }
    }
  }
</script>

<style lang="less" scoped>
  @import "chart";
</style>
