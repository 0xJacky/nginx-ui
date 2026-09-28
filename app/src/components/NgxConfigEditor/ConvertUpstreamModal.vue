<script setup lang="ts">
import type { SiteUpstreamContext } from './siteUpstreamContext'
import type { ManagedUpstreamDetail } from '@/api/upstream'
import upstreamApi from '@/api/upstream'
import { DEFAULT_ZONE_SIZE_KB, MIN_ZONE_SIZE_KB } from '@/views/upstream/zoneSize'
import { convertZoneChoice } from './convertZone'

const props = defineProps<{
  site: string
  upstream: string
  // Present in the site editor, which must be in sync with the file first.
  context?: SiteUpstreamContext
  // Parameters of the zone directive the block already declares; the group
  // keeps it and the zone switch is not offered.
  declaredZone?: string
}>()

const emit = defineEmits<{
  converted: [detail: ManagedUpstreamDetail]
}>()

const open = defineModel<boolean>('open', { default: false })

const { message } = App.useApp()

type Step = 'checking' | 'unsaved' | 'confirm'

const step = ref<Step>('confirm')
const isBusy = ref(false)

// Converted groups get a shared memory zone by default, like new groups.
const isZoneEnabled = ref(true)
const zoneSizeKb = ref<number | null>(DEFAULT_ZONE_SIZE_KB)
const hasDeclaredZone = computed(() => props.declaredZone !== undefined)

async function checkUnsaved() {
  if (!props.context) {
    step.value = 'confirm'
    return
  }
  step.value = 'checking'
  try {
    step.value = await props.context.hasUnsavedChanges() ? 'unsaved' : 'confirm'
  }
  catch {
    // Rendering the config failed; treat it as unsaved so nothing is lost.
    step.value = 'unsaved'
  }
}

watch(open, isOpen => {
  if (!isOpen)
    return
  isZoneEnabled.value = true
  zoneSizeKb.value = DEFAULT_ZONE_SIZE_KB
  checkUnsaved()
}, { immediate: true })

async function saveFirst() {
  isBusy.value = true
  try {
    await props.context!.save()
    await checkUnsaved()
  }
  catch {
    // The save error is already shown; stay on this step.
  }
  finally {
    isBusy.value = false
  }
}

async function discardFirst() {
  isBusy.value = true
  try {
    await props.context!.reload()
    await checkUnsaved()
  }
  finally {
    isBusy.value = false
  }
}

async function convert() {
  isBusy.value = true
  try {
    const detail = await upstreamApi.convertSiteUpstream({
      site: props.site,
      upstream: props.upstream,
      ...convertZoneChoice(props.declaredZone, isZoneEnabled.value, zoneSizeKb.value),
    })
    message.success($gettext('Upstream %{name} is now a shared upstream group', { name: props.upstream }))
    open.value = false
    emit('converted', detail)
  }
  catch {
    // The request layer shows the translated error (for example the nginx -t
    // output); both files were restored, so the dialog stays open.
  }
  finally {
    isBusy.value = false
  }
}
</script>

<template>
  <AModal
    v-model:open="open"
    :title="$gettext('Convert %{name} to a Shared Upstream Group', { name: upstream })"
    centered
    :width="560"
    :mask-closable="!isBusy"
  >
    <div
      v-if="step === 'checking'"
      class="py-6 text-center"
    >
      <ASpin />
    </div>

    <AAlert
      v-else-if="step === 'unsaved'"
      type="warning"
      show-icon
      :title="$gettext('This site has unsaved changes')"
      :description="$gettext('Converting rewrites the site file on the server. Save or discard your changes first, so they are neither lost nor written back over the converted file.')"
      data-testid="convert-upstream-unsaved"
    />

    <div
      v-else
      data-testid="convert-upstream-confirm"
    >
      <p class="mt-0">
        {{ $gettext('The upstream block %{name} moves out of %{site} into its own file, %{file}, and becomes an upstream group:', { name: upstream, site, file: `conf.d/upstream-${upstream}.conf` }) }}
      </p>
      <ul class="pl-5 mb-4">
        <li>{{ $gettext('It is listed and edited on the Upstream Groups page, with the health of its servers.') }}</li>
        <li>{{ $gettext('This site keeps proxying to it by name; nothing else in the site changes.') }}</li>
        <li>{{ $gettext('Other sites can choose it as their proxy target.') }}</li>
        <li>{{ $gettext('Comments inside the block are kept at the end of the group.') }}</li>
      </ul>
      <AForm
        layout="vertical"
        class="mb-2"
      >
        <AFormItem
          :label="$gettext('Shared Memory Zone')"
          :extra="hasDeclaredZone
            ? $gettext('The block already declares this zone; the group keeps it unchanged.')
            : $gettext('Keeps the group\'s state in shared memory as zone %{name}, so every worker process balances across the same servers and shares their failure counts. Without it each worker balances on its own.', { name: upstream })"
          data-testid="convert-upstream-zone"
        >
          <ATag
            v-if="hasDeclaredZone"
            class="font-mono"
            data-testid="convert-upstream-zone-kept"
          >
            zone {{ declaredZone }};
          </ATag>
          <AFlex
            v-else
            gap="middle"
            align="center"
            wrap
          >
            <ASwitch
              v-model:checked="isZoneEnabled"
              :aria-label="$gettext('Shared Memory Zone')"
              data-testid="convert-upstream-zone-enabled"
            />
            <ASpaceCompact v-if="isZoneEnabled">
              <AInputNumber
                v-model:value="zoneSizeKb"
                class="w-30"
                :min="MIN_ZONE_SIZE_KB"
                :precision="0"
                :aria-label="$gettext('Zone Size')"
                data-testid="convert-upstream-zone-size"
              />
              <ASpaceAddon>KB</ASpaceAddon>
            </ASpaceCompact>
          </AFlex>
        </AFormItem>
      </AForm>
      <p class="mb-0 text-gray-500 dark:text-gray-400">
        {{ $gettext('Nginx is tested once and reloaded. If the test or the reload fails, both files are restored.') }}
      </p>
    </div>

    <template #footer>
      <AButton
        :disabled="isBusy"
        @click="open = false"
      >
        {{ $gettext('Cancel') }}
      </AButton>
      <template v-if="step === 'unsaved'">
        <AButton
          danger
          :loading="isBusy"
          data-testid="convert-upstream-discard"
          @click="discardFirst"
        >
          {{ $gettext('Discard Changes') }}
        </AButton>
        <AButton
          type="primary"
          :loading="isBusy"
          data-testid="convert-upstream-save"
          @click="saveFirst"
        >
          {{ $gettext('Save Changes') }}
        </AButton>
      </template>
      <AButton
        v-else
        type="primary"
        :disabled="step !== 'confirm'"
        :loading="isBusy"
        data-testid="convert-upstream-submit"
        @click="convert"
      >
        {{ $gettext('Convert') }}
      </AButton>
    </template>
  </AModal>
</template>
