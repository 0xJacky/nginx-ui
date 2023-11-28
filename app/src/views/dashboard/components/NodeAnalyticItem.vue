<script setup lang="ts">
import Icon, { ArrowDownOutlined, ArrowUpOutlined, DatabaseOutlined, LineChartOutlined } from '@ant-design/icons-vue'
import cpu from '@/assets/svg/cpu.svg'
import memory from '@/assets/svg/memory.svg'
import { bytesToSize } from '@/lib/helper'
import UsageProgressLine from '@/components/Chart/UsageProgressLine.vue'

defineProps(['item'])
</script>

<template>
  <div class="hardware-monitor">
    <div class="hardware-monitor-item longer">
      <div>
        <LineChartOutlined />
        <span class="load-avg-describe">1min:</span>{{ ` ${item.avg_load?.load1?.toFixed(2)}` }} ·
        <span class="load-avg-describe">5min:</span>{{ item.avg_load?.load5?.toFixed(2) }} ·
        <span class="load-avg-describe">15min:</span>{{ item.avg_load?.load15?.toFixed(2) }}
      </div>
      <div>
        <ArrowUpOutlined />
        {{ bytesToSize(item?.network?.bytesSent) }}
        <ArrowDownOutlined />
        {{ bytesToSize(item?.network?.bytesRecv) }}
      </div>
    </div>
    <div class="hardware-monitor-item">
      <UsageProgressLine :percent="item.cpu_percent">
        <template #icon>
          <Icon :component="cpu" />
        </template>
        <span>{{ item.cpu_num }} CPU</span>
      </UsageProgressLine>
    </div>
    <div class="hardware-monitor-item">
      <UsageProgressLine :percent="item.memory_percent">
        <template #icon>
          <Icon :component="memory" />
        </template>
        <span>{{ item.memory_total }}</span>
      </UsageProgressLine>
    </div>
    <div class="hardware-monitor-item">
      <UsageProgressLine :percent="item.disk_percent">
        <template #icon>
          <DatabaseOutlined />
        </template>
        <span>{{ item.disk_total }}</span>
      </UsageProgressLine>
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
