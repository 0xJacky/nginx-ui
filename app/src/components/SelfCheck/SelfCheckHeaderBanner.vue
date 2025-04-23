<script setup lang="ts">
import { CloseCircleOutlined } from '@ant-design/icons-vue'
import { useElementSize } from '@vueuse/core'
import { storeToRefs } from 'pinia'
import { useSelfCheckStore } from './store'

const props = defineProps<{
  headerWeight?: number
  userWrapperWidth?: number
}>()

const router = useRouter()
const selfCheckStore = useSelfCheckStore()
const { hasError } = storeToRefs(selfCheckStore)

const alertEl = useTemplateRef('alertEl')
const { width: alertWidth } = useElementSize(alertEl)

const shouldHideAlert = computed(() => {
  if (!props.headerWeight || !props.userWrapperWidth || !alertWidth.value)
    return false
  return (props.headerWeight - props.userWrapperWidth - alertWidth.value - 60) < props.userWrapperWidth
})

const iconRightPosition = computed(() => {
  return props.userWrapperWidth ? `${props.userWrapperWidth + 50}px` : '50px'
})

onMounted(() => {
  selfCheckStore.check()
})
</script>

<template>
  <div v-show="hasError">
    <div ref="alertEl" class="self-check-alert" :style="{ visibility: shouldHideAlert ? 'hidden' : 'visible' }">
      <AAlert type="error" show-icon :message="$gettext('Self check failed, Nginx UI may not work properly')">
        <template #action>
          <AButton class="ml-4" size="small" danger @click="router.push('/system/self_check')">
            {{ $gettext('Check') }}
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
        :style="{ right: iconRightPosition }"
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
              {{ $gettext('Check') }}
            </AButton>
          </div>
        </div>
      </template>
    </APopover>
  </div>
</template>

<style lang="less" scoped>
.self-check-alert {
  position: absolute;
  left: 50%;
  top: 50%;
  transform: translate(-50%, -50%);
}

.error-icon {
  position: absolute;
  top: 50%;
  transform: translateY(-50%);
  color: #f5222d;
  cursor: pointer;
}
</style>
