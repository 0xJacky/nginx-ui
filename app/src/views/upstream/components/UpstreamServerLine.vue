<script setup lang="ts">
type ServerStatus = 'success' | 'error' | 'default' | 'warning'

const props = defineProps<{
  address: string
  isDown: boolean
  isBackup?: boolean
  weight?: number | null
  status: ServerStatus
  statusTitle: string
  isToggling?: boolean
  isReadOnly?: boolean
}>()

const emit = defineEmits<{
  toggle: [isEnabled: boolean]
}>()

const switchLabel = computed(() => props.isDown
  ? $gettext('Enable server %{address}', { address: props.address })
  : $gettext('Disable server %{address}', { address: props.address }))
</script>

<template>
  <div
    class="flex items-center gap-2 min-h-6"
    data-testid="upstream-server"
    :data-address="address"
    :title="statusTitle"
  >
    <!-- The switch sits first with a fixed size, so nothing around it moves
         when the pointer hovers or the state changes. -->
    <ASwitch
      size="small"
      class="shrink-0"
      :checked="!isDown"
      :loading="isToggling"
      :disabled="isReadOnly"
      :aria-label="switchLabel"
      data-testid="upstream-server-switch"
      @change="checked => emit('toggle', Boolean(checked))"
    />
    <ABadge
      :status
      :text="address"
      class="font-mono break-all"
      :class="{ 'line-through opacity-60': isDown }"
    />
    <ATag
      v-if="weight && weight !== 1"
      color="blue"
      class="me-0"
    >
      weight={{ weight }}
    </ATag>
    <ATag
      v-if="isBackup"
      color="orange"
      class="me-0"
    >
      backup
    </ATag>
    <ATag
      v-if="isDown"
      class="me-0"
    >
      down
    </ATag>
  </div>
</template>
