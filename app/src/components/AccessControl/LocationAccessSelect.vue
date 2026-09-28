<script setup lang="ts">
import { InheritValue, listOptions, PublicValue } from './options'
import { useAccessControlStore } from './store'

const props = defineProps<{
  serverIdx: number
  locationIdx: number
}>()

const store = useAccessControlStore()
const { lists, isApplying } = storeToRefs(store)

const server = computed(() => store.serverState(props.serverIdx))
const state = computed(() => store.locationState(props.serverIdx, props.locationIdx))
const isRestricted = computed(() => server.value?.mode === 'list')
const serverListName = computed(() => store.listName(server.value?.slug))

const isACMELocked = computed(() => state.value?.mode === 'acme')
const isManual = computed(() => state.value?.mode === 'manual' || server.value?.mode === 'manual')

const value = computed(() => {
  switch (state.value?.mode) {
    case 'list':
      return state.value.slug
    case 'public':
      return PublicValue
    default:
      return InheritValue
  }
})

const options = computed(() => [
  { label: $gettext('Inherit from server'), value: InheritValue },
  { label: $gettext('Public'), value: PublicValue },
  ...listOptions(lists.value, state.value?.slug),
])

// Spell out what applies, because a list in a location replaces the list of
// the server instead of adding to it.
const effect = computed(() => {
  const s = state.value
  if (!s)
    return ''
  switch (s.mode) {
    case 'acme':
      return $gettext('Kept public so certificate renewals can reach the ACME challenge.')
    case 'manual':
      return $gettext('This location has allow or deny rules that were not written by Nginx UI.')
    case 'public':
      return isRestricted.value
        ? $gettext('Public: overrides %{name} of the server and is reachable from anywhere.', { name: serverListName.value })
        : $gettext('Public.')
    case 'list': {
      const name = store.listName(s.slug)
      if (isRestricted.value && s.slug !== server.value?.slug)
        return $gettext('Only %{name} applies here. %{server} of the server does not apply to this location.', { name, server: serverListName.value })
      return $gettext('Only %{name} applies here.', { name })
    }
    default:
      return isRestricted.value
        ? $gettext('Uses %{name} from the server.', { name: serverListName.value })
        : $gettext('Public, like the server.')
  }
})

const isWarning = computed(() => (state.value?.mode === 'public' && isRestricted.value)
  || (state.value?.mode === 'list' && isRestricted.value && state.value.slug !== server.value?.slug))

function onChange(selected: unknown) {
  if (typeof selected !== 'string')
    return
  const base = { server: props.serverIdx, location: props.locationIdx }
  if (selected === InheritValue)
    store.apply([{ ...base, mode: 'inherit' }])
  else if (selected === PublicValue)
    store.apply([{ ...base, mode: 'public' }])
  else
    store.apply([{ ...base, mode: 'list', slug: selected }])
}

function unlockACME() {
  // Choosing a list explicitly is the only way to restrict an ACME location;
  // "inherit" would bring the exception back.
  onChange(server.value?.slug ?? PublicValue)
}
</script>

<template>
  <AFormItem v-if="state" :label="$gettext('Access')">
    <AFlex vertical gap="small">
      <AFlex v-if="isACMELocked" align="center" gap="small" wrap="wrap">
        <ASelect
          class="w-full max-w-100"
          :value="PublicValue"
          :options="[{ label: $gettext('Public (ACME challenge)'), value: PublicValue }]"
          disabled
        />
        <APopconfirm
          :title="$gettext('Restrict this location? Certificate renewals over HTTP-01 will fail.')"
          :ok-text="$gettext('Restrict')"
          :cancel-text="$gettext('Cancel')"
          @confirm="unlockACME"
        >
          <AButton type="link" size="small">
            {{ $gettext('Restrict anyway') }}
          </AButton>
        </APopconfirm>
      </AFlex>
      <ASelect
        v-else
        class="w-full max-w-100"
        :value="isManual ? undefined : value"
        :placeholder="isManual ? $gettext('Custom rules') : undefined"
        :options="options"
        :disabled="isManual"
        :loading="isApplying"
        data-testid="location-access-select"
        @change="onChange"
      />
      <ATypographyText :type="isWarning ? 'warning' : 'secondary'" class="text-xs">
        {{ effect }}
      </ATypographyText>
    </AFlex>
  </AFormItem>
</template>
