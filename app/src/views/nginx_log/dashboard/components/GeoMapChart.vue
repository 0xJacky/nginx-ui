<script setup lang="ts">
import type { ChinaMapData, WorldMapData } from '@/api/nginx_log'
import { useGeoTranslation } from '@/composables/useGeoTranslation'
import ChinaMapChart from './ChinaMapChart'
import WorldMapChart from './WorldMapChart'

const props = defineProps<{
  worldData: WorldMapData[] | null
  chinaData: ChinaMapData[] | null
  loading: boolean
}>()

const emit = defineEmits<{
  refresh: []
}>()

const { isChineseLocale } = useGeoTranslation()

// Map type selection - default to global, only allow china for Chinese locales
const mapType = ref<'global' | 'china'>('global')

// Watch language changes and reset to global if switching from Chinese to non-Chinese
watch(isChineseLocale, newVal => {
  if (!newVal && mapType.value === 'china') {
    mapType.value = 'global'
  }
})

// Segment options - only show China option for Chinese locales
const segmentOptions = computed(() => {
  const options = [{ label: $gettext('Global Map'), value: 'global' }]

  if (isChineseLocale.value) {
    options.push({ label: $gettext('China Map'), value: 'china' })
  }

  return options
})

// Show segment switcher only if there are multiple options
const showSegment = computed(() => {
  return isChineseLocale.value
})

// Card title
const cardTitle = computed(() => {
  return mapType.value === 'global' ? $gettext('Global Access Map') : $gettext('China Access Map')
})
</script>

<template>
  <ACard :loading="loading" class="geo-map-card">
    <template #title>
      <div class="flex items-center justify-between">
        <span>{{ cardTitle }}</span>
        <ASegmented
          v-if="showSegment"
          v-model:value="mapType"
          :options="segmentOptions"
        />
      </div>
    </template>

    <div class="geo-map-container">
      <!-- World Map -->
      <div v-show="mapType === 'global'">
        <WorldMapChart
          :data="props.worldData"
          :loading="props.loading"
          :hide-card="true"
          @refresh="emit('refresh')"
        />
      </div>

      <!-- China Map -->
      <div v-show="mapType === 'china'">
        <ChinaMapChart
          :data="props.chinaData"
          :loading="props.loading"
          :hide-card="true"
          @refresh="emit('refresh')"
        />
      </div>
    </div>
  </ACard>
</template>

<style scoped>
.geo-map-container {
  min-height: 400px;
}

/* Hide the inner card when used in this wrapper */
.geo-map-container :deep(.ant-card) {
  border: none;
  box-shadow: none;
  background: transparent;
  padding: 0;
}

.geo-map-container :deep(.ant-card .ant-card-head) {
  display: none;
}

.geo-map-container :deep(.ant-card .ant-card-body) {
  padding: 0;
}
</style>
