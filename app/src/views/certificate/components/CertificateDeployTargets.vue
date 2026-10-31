<script setup lang="ts">
import type { CertificateDeployTarget } from '@/api/cert_deploy'
import { CloudUploadOutlined } from '@antdv-next/icons'
import { deployCertificate, listCertificateDeployTargets } from '@/api/cert_deploy'
import { formatDateTime } from '@/lib/helper'
import { summarizeResults } from '../deployTargets'

// The deploy targets a certificate is pushed to, with the last outcome of
// each, and a button that pushes it to all of them now.
const props = defineProps<{
  certId: number
}>()

const { message } = useGlobalApp()

const loading = ref(false)
const deploying = ref(false)
const targets = ref<CertificateDeployTarget[]>([])

const hasEnabledTarget = computed(() => targets.value.some(item => item.target.enabled))

async function load() {
  if (!(props.certId > 0))
    return
  loading.value = true
  try {
    const res = await listCertificateDeployTargets(props.certId)
    targets.value = res.data ?? []
  }
  catch {
    targets.value = []
  }
  finally {
    loading.value = false
  }
}

watch(() => props.certId, load, { immediate: true })

async function handleDeploy() {
  deploying.value = true
  try {
    const res = await deployCertificate(props.certId)
    const { ok, failed } = summarizeResults(res.data ?? [])
    if (failed)
      message.warning($gettext('Deployed to %{ok} targets, %{failed} failed', { ok, failed }))
    else
      message.success($gettext('Deployed to %{ok} targets', { ok }))
  }
  catch {
    // The request layer shows the error.
  }
  finally {
    deploying.value = false
    await load()
  }
}

defineExpose({ reload: load })
</script>

<template>
  <ACard
    size="small"
    class="mb-4"
    :title="$gettext('Deploy Targets')"
    :loading="loading"
  >
    <template #extra>
      <AFlex :gap="8" align="center">
        <RouterLink to="/certificates/deploy_targets">
          {{ $gettext('Manage') }}
        </RouterLink>
        <AButton
          v-if="targets.length"
          type="primary"
          size="small"
          :disabled="!hasEnabledTarget"
          :loading="deploying"
          @click="handleDeploy"
        >
          <template #icon>
            <CloudUploadOutlined />
          </template>
          {{ $gettext('Deploy Now') }}
        </AButton>
      </AFlex>
    </template>

    <div v-if="!targets.length" class="text-sm text-gray-500">
      {{ $gettext('This certificate is not pushed to any external target.') }}
    </div>
    <AFlex v-else vertical :gap="12">
      <AFlex
        v-for="item in targets"
        :key="item.target.id"
        vertical
        :gap="4"
      >
        <AFlex justify="space-between" align="center" wrap="wrap" :gap="8">
          <span class="font-500">
            {{ item.target.name }}
            <span class="text-gray-500 font-normal">· {{ item.kind_name || item.target.kind }}</span>
          </span>
          <AFlex :gap="4" align="center">
            <ATag v-if="!item.target.enabled">
              {{ $gettext('Disabled') }}
            </ATag>
            <ATag v-if="!item.kind_name" color="orange">
              {{ $gettext('Plugin unavailable') }}
            </ATag>
            <ATag v-if="item.last" :color="item.last.status === 'ok' ? 'green' : 'red'">
              {{ item.last.status === 'ok' ? $gettext('Success') : $gettext('Failed') }}
            </ATag>
            <ATag v-else>
              {{ $gettext('Never deployed') }}
            </ATag>
          </AFlex>
        </AFlex>
        <div v-if="item.last" class="text-xs text-gray-500 break-all">
          {{ formatDateTime(item.last.created_at) }}
          <template v-if="item.last.attempts > 1">
            · {{ $gettext('%{count} attempts', { count: item.last.attempts }) }}
          </template>
          <template v-if="item.last.message">
            · {{ item.last.message }}
          </template>
        </div>
      </AFlex>
    </AFlex>
  </ACard>
</template>
