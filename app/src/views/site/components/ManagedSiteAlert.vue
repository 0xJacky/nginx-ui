<script setup lang="ts">
import { managedIngressName } from '@/views/site/managedSite'

const props = defineProps<{
  managedBy?: string | null
  banner?: boolean
}>()

// Unlocks the actions the site API refuses for a managed site; they are then
// sent with the force flag.
const force = defineModel<boolean>('force', { default: false })

const ingressName = computed(() => managedIngressName(props.managedBy))
</script>

<template>
  <AAlert
    v-if="ingressName"
    :type="force ? 'error' : 'warning'"
    :banner
    show-icon
    data-testid="managed-site-alert"
  >
    <template #title>
      {{ $gettext('Managed by ingress %{name}', { name: ingressName }) }}
    </template>
    <template #description>
      <div>
        {{ $gettext('Changes will be overwritten by the next reconcile. Edit the ingress in its controller instead.') }}
      </div>
      <AFlex
        wrap
        align="center"
        gap="middle"
        class="mt-2"
      >
        <RouterLink :to="{ path: '/ingresses', query: { name: ingressName } }">
          {{ $gettext('View ingress') }}
        </RouterLink>
        <label class="inline-flex items-center gap-2 cursor-pointer">
          <ASwitch
            v-model:checked="force"
            size="small"
            data-testid="managed-site-force"
          />
          <span>{{ $gettext('Edit anyway') }}</span>
        </label>
      </AFlex>
    </template>
  </AAlert>
</template>
