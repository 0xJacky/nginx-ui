<script setup lang="ts">
import { CloseCircleOutlined } from '@antdv-next/icons'
import { useElementSize } from '@vueuse/core'
import { storeToRefs } from 'pinia'
import { useSelfCheckStore } from './store'

const props = defineProps<{
  /** Room left in the header once the breadcrumb and the icons are placed. */
  availableWidth?: number
}>()

const router = useRouter()
const selfCheckStore = useSelfCheckStore()
const { hasError, loading, data } = storeToRefs(selfCheckStore)

const alertEl = useTemplateRef('alertEl')
const { width: alertWidth } = useElementSize(alertEl)

// The full alert stays rendered while collapsed, taken out of the flow, so its
// width is still known when deciding whether it fits again.
const shouldHideAlert = computed(() => props.availableWidth !== undefined
  && alertWidth.value > 0
  && alertWidth.value > props.availableWidth)

const allFailingAreFixable = computed(() => {
  const failing = data.value?.filter(r => r.status === 'error') ?? []
  return failing.length > 0 && failing.every(r => r.fixable)
})

const actionLabel = computed(() =>
  allFailingAreFixable.value ? $gettext('Fix') : $gettext('Check'),
)

onMounted(() => {
  selfCheckStore.check()
})
</script>

<template>
  <div v-show="hasError && !loading">
    <div ref="alertEl" class="self-check-alert" :class="{ measuring: shouldHideAlert }">
      <AAlert type="error" show-icon :title="$gettext('Self check failed, Nginx UI may not work properly')">
        <template #action>
          <AButton class="ml-4" size="small" danger @click="router.push('/system/self_check')">
            {{ actionLabel }}
          </AButton>
        </template>
      </AAlert>
    </div>

    <APopover
      v-if="shouldHideAlert"
      placement="bottomRight"
      trigger="hover"
    >
      <CloseCircleOutlined
        class="error-icon"
        @click="router.push('/system/self_check')"
      />
      <template #content>
        <div class="flex items-center gap-2">
          <CloseCircleOutlined class="text-red-500" />
          <div>
            {{ $gettext('Self check failed, Nginx UI may not work properly') }}
          </div>
          <div>
            <AButton size="small" danger @click="router.push('/system/self_check')">
              {{ actionLabel }}
            </AButton>
          </div>
        </div>
      </template>
    </APopover>
  </div>
</template>

<style lang="less" scoped>
.self-check-alert.measuring {
  position: absolute;
  visibility: hidden;
  pointer-events: none;
}

.error-icon {
  display: flex;
  font-size: 16px;
  color: #f5222d;
  cursor: pointer;
}
</style>
