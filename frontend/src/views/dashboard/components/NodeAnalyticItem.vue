<script setup lang="ts">
import cpu from '@/assets/svg/cpu.svg'
import memory from '@/assets/svg/memory.svg'
import {bytesToSize} from '@/lib/helper'
import Icon, {ArrowDownOutlined, ArrowUpOutlined, DatabaseOutlined, LineChartOutlined} from '@ant-design/icons-vue'
import UsageProgressLine from '@/components/Chart/UsageProgressLine.vue'

const props = defineProps(['item'])
</script>

<template>
  <div class="hardware-monitor">
    <div class="hardware-monitor-item longer">
      <div>
        <line-chart-outlined/>
        <span class="load-avg-describe">1min:</span>{{ ' ' + item.avg_load?.load1?.toFixed(2) }} ·
        <span class="load-avg-describe">5min:</span>{{ item.avg_load?.load5?.toFixed(2) }} ·
        <span class="load-avg-describe">15min:</span>{{ item.avg_load?.load15?.toFixed(2) }}
      </div>
      <div>
        <arrow-up-outlined/>
        {{ bytesToSize(item?.network?.bytesSent) }}
        <arrow-down-outlined/>
        {{ bytesToSize(item?.network?.bytesRecv) }}
      </div>
    </div>
    <div class="hardware-monitor-item">
      <usage-progress-line :percent="item.cpu_percent">
        <template #icon>
          <Icon :component="cpu"/>
        </template>
        <span>{{ item.cpu_num }} CPU</span>
      </usage-progress-line>
    </div>
    <div class="hardware-monitor-item">
      <usage-progress-line :percent="item.memory_percent">
        <template #icon>
          <Icon :component="memory"/>
        </template>
        <span>{{ item.memory_total }}</span>
      </usage-progress-line>
    </div>
    <div class="hardware-monitor-item">
      <usage-progress-line :percent="item.disk_percent">
        <template #icon>
          <database-outlined/>
        </template>
        <span>{{ item.disk_total }}</span>
      </usage-progress-line>
    </div>
  </div>
</template>

<style scoped lang="less">
.hardware-monitor {
  display: flex;

  @media (max-width: 900px) {
    display: block;
  }

  .hardware-monitor-item {
    width: 150px;
    margin-right: 30px;
    @media (max-width: 900px) {
      margin-bottom: 5px;
    }
  }

  .longer {
    width: 300px;
  }
}

.load-avg-describe {
  @media (max-width: 1200px) and  (min-width: 600px) {
    display: none;
  }
}
</style>
