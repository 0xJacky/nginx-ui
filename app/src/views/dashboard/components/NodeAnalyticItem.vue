<script setup lang="ts">
import type { AnalyticNode } from '@/api/node'
import Icon, { ArrowDownOutlined, ArrowUpOutlined, DatabaseOutlined, LineChartOutlined, SendOutlined } from '@ant-design/icons-vue'
import cpu from '@/assets/svg/cpu.svg?component'
import memory from '@/assets/svg/memory.svg?component'
import UsageProgressLine from '@/components/Chart/UsageProgressLine.vue'
import { bytesToSize } from '@/lib/helper'

defineProps<{
  item: AnalyticNode
  currentNodeId?: number
  localVersion?: string
  onLinkStart?: (item: AnalyticNode) => void
}>()
</script>

<template>
  <div class="node-analytics-container">
    <div class="hardware-monitor justify-between w-full">
      <div class="hardware-monitor-item longer <lg:mb-2">
        <div class="mb-1">
          <LineChartOutlined class="mr-1" />
          <span class="load-avg-describe">1min:</span>{{ item.avg_load?.load1?.toFixed(2) }} ·
          <span class="load-avg-describe">5min:</span>{{ item.avg_load?.load5?.toFixed(2) }} ·
          <span class="load-avg-describe">15min:</span>{{ item.avg_load?.load15?.toFixed(2) }}
        </div>
        <div class="flex">
          <div class="sm:text-sm md:text-xs lg:text-sm">
            <ArrowUpOutlined />
            {{ bytesToSize(item?.network?.bytesSent || 0) }}
          </div>
          <div class="ml-2 sm:text-sm md:text-xs lg:text-sm">
            <ArrowDownOutlined />
            {{ bytesToSize(item?.network?.bytesRecv || 0) }}
          </div>
        </div>
      </div>
      <div class="flex justify-between">
        <div class="<md:block md:flex">
          <div class="hardware-monitor-item">
            <UsageProgressLine :percent="item.cpu_percent || 0">
              <template #icon>
                <Icon :component="cpu" />
              </template>
              <span>{{ item.cpu_num || 0 }} CPU</span>
            </UsageProgressLine>
          </div>
          <div class="hardware-monitor-item">
            <UsageProgressLine :percent="item.memory_percent || 0">
              <template #icon>
                <Icon :component="memory" />
              </template>
              <span>{{ item.memory_total || 'N/A' }}</span>
            </UsageProgressLine>
          </div>
          <div class="hardware-monitor-item">
            <UsageProgressLine :percent="item.disk_percent || 0">
              <template #icon>
                <DatabaseOutlined />
              </template>
              <span>{{ item.disk_total || 'N/A' }}</span>
            </UsageProgressLine>
          </div>
        </div>

        <!-- Link button section -->
        <div class="link-button-section">
          <AButton
            v-if="item.version === localVersion"
            type="primary"
            :disabled="!item.status || currentNodeId === item.id"
            ghost
            class="link-btn"
            @click="onLinkStart?.(item)"
          >
            <SendOutlined />
            <span class="link-btn-text">
              {{ currentNodeId !== item.id ? $gettext('Link') : $gettext('Connected') }}
            </span>
          </AButton>
          <ATooltip
            v-else
            placement="topLeft"
          >
            <template #title>
              {{ $gettext('The remote Nginx UI version is not compatible with the local Nginx UI version. '
                + 'To avoid potential errors, please upgrade the remote Nginx UI to match the local version.') }}
            </template>
            <AButton
              ghost
              disabled
              class="link-btn"
            >
              <SendOutlined />
              <span class="link-btn-text">{{ $gettext('Link') }}</span>
            </AButton>
          </ATooltip>
        </div>
      </div>
    </div>
  </div>
</template>

<style scoped lang="less">
.node-analytics-container {
  display: flex;
  justify-content: space-between;
  width: 100%;
  position: relative; // For absolute positioned button on mobile

  @media (max-width: 768px) {
    flex-direction: column;
    gap: 16px;
  }
}

.hardware-monitor {
  display: flex;
  flex: 1;

  @media (max-width: 1000px) {
    display: block;
  }

  @media (min-width: 768px) and (max-width: 1000px) {
    display: flex;
    flex-wrap: wrap;
    gap: 12px;
  }

  .hardware-monitor-item {
    width: 140px;
    margin-right: 20px;

    @media(min-width: 2100px) {
      width: 400px;
      margin-right: 50px;
    }

    @media(min-width: 1800px) and (max-width: 2100px) {
      width: 300px;
      margin-right: 20px;
    }

    @media(min-width: 1600px) and (max-width: 1800px) {
      width: 270px;
      margin-right: 20px;
    }

    @media(min-width: 1500px) and (max-width: 1600px) {
      width: 220px;
      margin-right: 30px;
    }

    @media(min-width: 1400px) and (max-width: 1500px) {
      width: 180px;
      margin-right: 25px;
    }

    @media(min-width: 1000px) and (max-width: 1400px) {
      width: 150px;
      margin-right: 16px;
    }

    @media(min-width: 768px) and (max-width: 1000px) {
      width: 140px;
      margin-right: 16px;
      margin-bottom: 12px;
    }

    @media(min-width: 400px) and (max-width: 768px) {
      width: 280px;
    }

    @media(max-width: 400px) {
      width: 200px;
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
  @media (min-width: 768px) and (max-width: 1000px) {
    .longer {
      width: 200px;
      flex-shrink: 0;
    }
  }
  @media (min-width: 1000px) and (max-width: 1100px) {
    .longer {
      min-width: 180px;
      .load-avg-describe {
        display: none;
      }
    }
  }
  @media (min-width: 1100px) and (max-width: 1200px) {
    .longer {
      min-width: 200px;
    }
  }
  @media (min-width: 1200px) and (max-width: 1400px) {
    .longer {
      min-width: 240px;
    }
  }
  @media (min-width: 1400px) {
    .longer {
      min-width: 300px;
    }
  }
  @media (min-width: 400px) and (max-width: 768px) {
    .longer {
      min-width: 250px;
    }
  }
}

.link-button-section {
  display: flex;
  align-items: center;
  margin-left: 16px;
  flex-shrink: 0;

  .link-btn {
    min-width: 80px;
    height: 32px;
    display: flex;
    align-items: center;
    justify-content: center;
    gap: 4px;

    .link-btn-text {
      display: inline;

      @media (max-width: 480px) {
        display: none;
      }

      @media(min-width: 1000px) and (max-width: 1400px) {
        display: none;
      }
    }

    @media(min-width: 1000px) and (max-width: 1400px) {
      min-width: 32px;
      padding: 0 8px;
    }

    @media (max-width: 480px) {
      min-width: 32px;
      padding: 0 8px;
    }
  }

  @media (min-width: 768px) and (max-width: 1000px) {
    margin-left: 8px;
    align-self: flex-start;
    margin-top: 8px;
  }

  @media (max-width: 768px) {
    margin-left: 0;
    align-self: center;
  }
}
</style>
