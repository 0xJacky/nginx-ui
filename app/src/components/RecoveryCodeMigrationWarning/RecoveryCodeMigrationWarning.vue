<script setup lang="ts">
import { WarningOutlined } from '@ant-design/icons-vue'
import { useElementSize } from '@vueuse/core'
import { useUserStore } from '@/pinia'

const props = defineProps<{
  headerWeight?: number
  userWrapperWidth?: number
}>()

const router = useRouter()
const userStore = useUserStore()
const { twoFAStatus } = storeToRefs(userStore)

const alertEl = useTemplateRef('alertEl')
const { width: alertWidth } = useElementSize(alertEl)

const hasMigrationWarning = computed(() => twoFAStatus.value.recovery_codes_migration_required)

const shouldHideAlert = computed(() => {
  if (!props.headerWeight || !props.userWrapperWidth || !alertWidth.value)
    return false
  return (props.headerWeight - props.userWrapperWidth - alertWidth.value - 60) < props.userWrapperWidth
})

const iconRightPosition = computed(() => {
  return props.userWrapperWidth ? `${props.userWrapperWidth + 82}px` : '82px'
})

function openRecoveryCodes() {
  router.push('/profile')
}
</script>

<template>
  <div v-show="hasMigrationWarning">
    <div ref="alertEl" class="migration-alert" :style="{ visibility: shouldHideAlert ? 'hidden' : 'visible' }">
      <AAlert
        type="warning"
        show-icon
        :message="$gettext('Legacy recovery code is deprecated. Generate new recovery codes to keep account recovery secure.')"
      >
        <template #action>
          <AButton class="ml-4" size="small" @click="openRecoveryCodes">
            {{ $gettext('Generate') }}
          </AButton>
        </template>
      </AAlert>
    </div>

    <APopover
      v-if="shouldHideAlert"
      placement="bottomRight"
      trigger="hover"
    >
      <WarningOutlined
        class="warning-icon"
        :style="{ right: iconRightPosition }"
        @click="openRecoveryCodes"
      />
      <template #content>
        <div class="flex items-center gap-2">
          <WarningOutlined class="text-yellow-500" />
          <div>
            {{ $gettext('Legacy recovery code is deprecated. Generate new recovery codes to keep account recovery secure.') }}
          </div>
          <div>
            <AButton size="small" @click="openRecoveryCodes">
              {{ $gettext('Generate') }}
            </AButton>
          </div>
        </div>
      </template>
    </APopover>
  </div>
</template>

<style lang="less" scoped>
.migration-alert {
  position: absolute;
  left: 50%;
  top: 50%;
  transform: translate(-50%, -50%);
}

.warning-icon {
  position: absolute;
  top: 50%;
  transform: translateY(-50%);
  color: #faad14;
  cursor: pointer;
}
</style>
