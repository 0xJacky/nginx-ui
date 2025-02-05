<script setup lang="ts">
import cpu from '@/assets/svg/cpu.svg?component'
import memory from '@/assets/svg/memory.svg?component'
import UsageProgressLine from '@/components/Chart/UsageProgressLine.vue'
import { bytesToSize } from '@/lib/helper'
import Icon, { ArrowDownOutlined, ArrowUpOutlined, DatabaseOutlined, LineChartOutlined } from '@ant-design/icons-vue'

defineProps<{
  item: {
    avg_load: {
      load1: number
      load5: number
      load15: number
    }
    network: {
      bytesSent: number
      bytesRecv: number
    }
    cpu_percent: number
    cpu_num: number
    memory_percent: number
    memory_total: string
    disk_percent: number
    disk_total: string
  }
}>()
</script>

<template>
  <div class="hardware-monitor">
    <div class="hardware-monitor-item longer">
      <div class="mb-1">
        <LineChartOutlined class="mr-1" />
        <span class="load-avg-describe">1min:</span>{{ item.avg_load?.load1?.toFixed(2) }} ·
        <span class="load-avg-describe">5min:</span>{{ item.avg_load?.load5?.toFixed(2) }} ·
        <span class="load-avg-describe">15min:</span>{{ item.avg_load?.load15?.toFixed(2) }}
      </div>
      <div class="flex">
        <div class="sm:text-sm md:text-xs lg:text-sm">
          <ArrowUpOutlined />
          {{ bytesToSize(item?.network?.bytesSent) }}
        </div>
        <div class="ml-2 sm:text-sm md:text-xs lg:text-sm">
          <ArrowDownOutlined />
          {{ bytesToSize(item?.network?.bytesRecv) }}
        </div>
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

  @media (max-width: 1000px) {
    display: block;
  }

  .hardware-monitor-item {
    width: 140px;
    margin-right: 20px;

    @media(min-width: 1800px) {
      width: 300px;
      margin-bottom: 10px;
      margin-right: 50px;
    }

    @media(min-width: 1600px) and (max-width: 1800px) {
      width: 270px;
      margin-bottom: 10px;
      margin-right: 20px;
    }

    @media(min-width: 1500px) and (max-width: 1600px) {
      width: 230px;
      margin-bottom: 10px;
      margin-right: 30px;
    }

    @media(min-width: 1400px) and (max-width: 1500px) {
      width: 180px;
      margin-bottom: 10px;
      margin-right: 25px;
    }

    @media(min-width: 400px) and (max-width: 1000px) {
      width: 280px;
      margin-bottom: 10px;
    }

    @media(max-width: 400px) {
      width: 200px;
      margin-bottom: 10px;
    }
  }

  .longer {
    width: 180px;
  }

  .load-avg-describe {
    margin-right: 2px;
  }

  @media (max-width: 400px) {
    .longer {
      width: 180px;
      .load-avg-describe {
        display: none;
      }
    }
  }
  @media (min-width: 400px) and (max-width: 500px) {
    .longer {
      width: 300px;
    }
  }
  @media (min-width: 1400px) {
    .longer {
      min-width: 300px;
    }
  }
  @media (min-width: 1200px) {
    .longer {
      min-width: 270px;
    }
  }
  @media (min-width: 400px) and (max-width: 1000px) {
    .longer {
      min-width: 250px;
    }
  }
  @media (min-width: 1100px) and (max-width: 1200px) {
    .longer {
      min-width: 200px;
    }
  }
}
</style>
