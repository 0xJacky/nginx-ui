<script setup lang="ts">
import { CheckCircleOutlined, CloseCircleOutlined, WarningOutlined } from '@ant-design/icons-vue'
import GeoLiteDownload from '@/components/GeoLiteDownload'
import AsyncErrorDisplay from './AsyncErrorDisplay.vue'
import { useSelfCheckStore } from './store'

const store = useSelfCheckStore()

const { data, loading, fixing } = storeToRefs(store)

const geoLiteModalVisible = ref(false)

function handleFix(key: string) {
  if (key === 'GeoLite-DB') {
    geoLiteModalVisible.value = true
  }
  else {
    store.fix(key)
  }
}

function handleGeoLiteDownloadComplete() {
  geoLiteModalVisible.value = false
  store.check()
}

onMounted(() => {
  store.check()
})
</script>

<template>
  <ACard :title="$gettext('Self Check')">
    <template #extra>
      <AButton
        type="link"
        size="small"
        :loading="loading"
        @click="store.check"
      >
        {{ $gettext('Recheck') }}
      </AButton>
    </template>
    <AList>
      <AListItem v-for="(item, index) in data" :key="index">
        <template v-if="item.status === 'error' && item.fixable" #actions>
          <AButton type="link" size="small" :loading="fixing[item.key]" @click="handleFix(item.key)">
            {{ $gettext('Attempt to fix') }}
          </AButton>
        </template>
        <AListItemMeta>
          <template #title>
            {{ item.name?.() }}
          </template>
          <template #description>
            <div>
              {{ item.description?.() }}
            </div>
            <div v-if="item.status !== 'success' && item.err?.message" class="mt-1">
              <Suspense>
                <AsyncErrorDisplay :error="item.err" :status="item.status" />
                <template #fallback>
                  <ATag :color="item.status === 'warning' ? 'warning' : 'error'">
                    {{ item.err.message }}
                  </ATag>
                </template>
              </Suspense>
            </div>
          </template>
          <template #avatar>
            <div class="text-23px">
              <CheckCircleOutlined v-if="item.status === 'success'" class="text-green" />
              <WarningOutlined v-else-if="item.status === 'warning'" class="text-yellow" />
              <CloseCircleOutlined v-else class="text-red" />
            </div>
          </template>
        </AListItemMeta>
      </AListItem>
    </AList>

    <AModal
      v-model:open="geoLiteModalVisible"
      :title="$gettext('Download GeoLite2 Database')"
      :footer="null"
      width="600px"
    >
      <GeoLiteDownload @download-complete="handleGeoLiteDownloadComplete" />
    </AModal>
  </ACard>
</template>

<style scoped lang="less">
:deep(.ant-list-item-meta) {
  align-items: center !important;
}

.text-yellow {
  color: #faad14;
}
</style>
