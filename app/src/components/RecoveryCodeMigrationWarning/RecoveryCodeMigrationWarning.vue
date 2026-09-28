<script setup lang="ts">
import { WarningOutlined } from '@antdv-next/icons'
import { useElementSize } from '@vueuse/core'
import { useUserStore } from '@/pinia'

const props = defineProps<{
  /** Room left in the header once the breadcrumb and the icons are placed. */
  availableWidth?: number
}>()

const router = useRouter()
const userStore = useUserStore()
const { twoFAStatus } = storeToRefs(userStore)

const alertEl = useTemplateRef('alertEl')
const { width: alertWidth } = useElementSize(alertEl)

const hasMigrationWarning = computed(() => twoFAStatus.value.recovery_codes_migration_required)

// The full alert stays rendered while collapsed, taken out of the flow, so its
// width is still known when deciding whether it fits again.
const shouldHideAlert = computed(() => props.availableWidth !== undefined
  && alertWidth.value > 0
  && alertWidth.value > props.availableWidth)

// The header centers the banners only while one shows the full alert; a lone
// icon stays next to the header icons.
const isExpanded = computed(() => hasMigrationWarning.value && !shouldHideAlert.value)

defineExpose({ isExpanded })

function openRecoveryCodes() {
  router.push('/profile')
}
</script>

<template>
  <div v-show="hasMigrationWarning">
    <div ref="alertEl" class="migration-alert" :class="{ measuring: shouldHideAlert }">
      <AAlert
        type="warning"
        show-icon
        :title="$gettext('Legacy recovery code is deprecated. Generate new recovery codes to keep account recovery secure.')"
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
.migration-alert.measuring {
  position: absolute;
  visibility: hidden;
  pointer-events: none;
}

.warning-icon {
  display: flex;
  font-size: 16px;
  color: #faad14;
  cursor: pointer;
}
</style>
