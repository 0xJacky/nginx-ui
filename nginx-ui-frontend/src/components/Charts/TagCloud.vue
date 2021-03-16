<template>
  <v-chart :width="width" :height="height" :padding="[0]" :data="data" :scale="scale">
    <v-tooltip :show-title="false" />
    <v-coord type="rect" direction="TL" />
    <v-point position="x*y" color="category" shape="cloud" tooltip="value*category" />
  </v-chart>
</template>

<script>
import { registerShape } from 'viser-vue'
const DataSet = require('@antv/data-set')

const imgUrl = 'https://gw.alipayobjects.com/zos/rmsportal/gWyeGLCdFFRavBGIDzWk.png'

const scale = [
  { dataKey: 'x', nice: false },
  { dataKey: 'y', nice: false }
]

registerShape('point', 'cloud', {
  draw (cfg, container) {
    return container.addShape('text', {
      attrs: {
        fillOpacity: cfg.opacity,
        fontSize: cfg.origin._origin.size,
        rotate: cfg.origin._origin.rotate,
        text: cfg.origin._origin.text,
        textAlign: 'center',
        fontFamily: cfg.origin._origin.font,
        fill: cfg.color,
        textBaseline: 'Alphabetic',
        ...cfg.style,
        x: cfg.x,
        y: cfg.y
      }
    })
  }
})

export default {
  name: 'TagCloud',
  props: {
    tagList: {
      type: Array,
      required: true
    },
    height: {
      type: Number,
      default: 400
    },
    width: {
      type: Number,
      default: 640
    }
  },
  data () {
    return {
      data: [],
      scale
    }
  },
  watch: {
    tagList: function (val) {
      if (val.length > 0) {
        this.initTagCloud(val)
      }
    }
  },
  mounted () {
    if (this.tagList.length > 0) {
      this.initTagCloud(this.tagList)
    }
  },
  methods: {
    initTagCloud (dataSource) {
      const { height, width } = this

      const dv = new DataSet.View().source(dataSource)
      const range = dv.range('value')
      const min = range[0]
      const max = range[1]
      const imageMask = new Image()
      imageMask.crossOrigin = ''
      imageMask.src = imgUrl
      imageMask.onload = () => {
        dv.transform({
          type: 'tag-cloud',
          fields: ['name', 'value'],
          size: [width, height],
          imageMask,
          font: 'Verdana',
          padding: 0,
          timeInterval: 5000, // max execute time
          rotate () {
            let random = ~~(Math.random() * 4) % 4
            if (random === 2) {
              random = 0
            }
            return random * 90 // 0, 90, 270
          },
          fontSize (d) {
            if (d.value) {
              return ((d.value - min) / (max - min)) * (32 - 8) + 8
            }
            return 0
          }
        })
        this.data = dv.rows
      }
    }
  }
}
</script>
