<script setup lang="ts">
import type { SelfCheckAccessOptions } from '@/api/self_check'
import { CheckCircleOutlined, CloseCircleOutlined, WarningOutlined } from '@antdv-next/icons'
import GeoLiteDownload from '@/components/GeoLiteDownload'
import { List, ListItem, ListItemMeta } from '@/components/List'
import AsyncErrorDisplay from './AsyncErrorDisplay.vue'
import { useSelfCheckStore } from './store'

const props = withDefaults(defineProps<{
  installSecret?: string
  setupAuth?: boolean
  frontendDebug?: boolean
}>(), {
  setupAuth: false,
  frontendDebug: false,
})

const store = useSelfCheckStore()

const { data, loading, fixing, accessError } = storeToRefs(store)

const geoLiteModalVisible = ref(false)

const accessOptions = computed<SelfCheckAccessOptions | undefined>(() => {
  if (!props.setupAuth) {
    return undefined
  }

  return {
    setupAuth: true,
    installSecret: props.installSecret,
    debugMode: props.frontendDebug ? 'frontend' : undefined,
  }
})

function handleFix(key: string) {
  if (key === 'GeoLite-DB') {
    geoLiteModalVisible.value = true
  }
  else {
    store.fix(key, accessOptions.value)
  }
}

function handleGeoLiteDownloadComplete() {
  geoLiteModalVisible.value = false
  store.check(accessOptions.value)
}

watch(accessOptions, options => {
  store.check(options)
}, { immediate: true })
</script>

<template>
  <ACard :title="$gettext('Self Check')">
    <template #extra>
      <AButton
        type="link"
        size="small"
        :disabled="setupAuth && !installSecret"
        :loading="loading"
        @click="store.check(accessOptions)"
      >
        {{ $gettext('Recheck') }}
      </AButton>
    </template>
    <AAlert
      v-if="setupAuth && !installSecret"
      type="info"
      show-icon
      class="mb-4"
      :title="$gettext('Enter the install secret to run the system check.')"
    />
    <AAlert
      v-else-if="accessError"
      type="error"
      show-icon
      class="mb-4"
      :title="accessError"
    />
    <List :data-source="data" item-layout="horizontal">
      <template #renderItem="{ item }">
        <ListItem>
          <template v-if="item.status === 'error' && item.fixable" #actions>
            <AButton type="link" size="small" :loading="fixing[item.key]" @click="handleFix(item.key)">
              {{ $gettext('Attempt to fix') }}
            </AButton>
          </template>
          <ListItemMeta>
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
          </ListItemMeta>
        </ListItem>
      </template>
    </List>

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
:deep(.nui-list-item-meta) {
  align-items: center !important;
}

.text-yellow {
  color: #faad14;
}
</style>
