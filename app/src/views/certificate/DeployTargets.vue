<script setup lang="tsx">
import type { CustomRenderArgs, StdTableColumn } from '@uozi-admin/curd'
import type { CertDeployTarget } from '@/api/cert_deploy'
import { datetimeRender, StdCurd } from '@uozi-admin/curd'
import { Tag } from 'antdv-next'
import certDeployTarget, { deployTarget } from '@/api/cert_deploy'
import DeployHistory from './components/DeployHistory.vue'
import DeployTargetEditor from './components/DeployTargetEditor.vue'
import {
  certificateLabel,
  deployKindLabel,
  loadCertificateOptions,
  loadDeployKinds,
  summarizeResults,
} from './deployTargets'

const { message } = useGlobalApp()
const deployingStates = ref<Record<number, boolean>>({})

const historyTarget = ref<CertDeployTarget>()
const historyOpen = ref(false)

onMounted(() => {
  loadDeployKinds(true)
  loadCertificateOptions(true)
})

function showHistory(record: CertDeployTarget) {
  historyTarget.value = record
  historyOpen.value = true
}

async function handleDeploy(record: CertDeployTarget) {
  if (!record.id)
    return
  deployingStates.value[record.id] = true
  try {
    const res = await deployTarget(record.id)
    const results = res.data ?? []
    const { ok, failed } = summarizeResults(results)
    if (!results.length)
      message.info($gettext('No certificate is bound to this target'))
    else if (failed)
      message.warning($gettext('Deployed %{ok} certificates, %{failed} failed', { ok, failed }))
    else
      message.success($gettext('Deployed %{ok} certificates', { ok }))
  }
  catch {
    // The request layer shows the error.
  }
  finally {
    deployingStates.value[record.id] = false
  }
}

const columns: StdTableColumn[] = [
  {
    title: () => $gettext('Name'),
    dataIndex: 'name',
    sorter: true,
    pure: true,
    search: true,
    edit: {
      type: 'input',
      formItem: {
        required: true,
      },
    },
  },
  {
    title: () => $gettext('Target Type'),
    dataIndex: 'kind',
    customRender: ({ text }: CustomRenderArgs) => deployKindLabel(text as string),
    edit: {
      type: (context: { formData: CertDeployTarget }) => {
        if (context.formData.enabled === undefined)
          context.formData.enabled = true
        return <DeployTargetEditor v-model={context.formData} />
      },
      formItem: {
        hiddenLabelInEdit: true,
      },
    },
    pure: true,
  },
  {
    title: () => $gettext('Certificate'),
    dataIndex: 'cert_id',
    customRender: ({ text }: CustomRenderArgs) => certificateLabel(text as number),
    hiddenInEdit: true,
    pure: true,
  },
  {
    title: () => $gettext('Configuration'),
    dataIndex: 'config',
    hiddenInTable: true,
    hiddenInEdit: true,
  },
  {
    title: () => $gettext('Status'),
    dataIndex: 'enabled',
    customRender: ({ text }: CustomRenderArgs) => {
      return text
        ? <Tag color="green">{$gettext('Enabled')}</Tag>
        : <Tag color="red">{$gettext('Disabled')}</Tag>
    },
    edit: {
      type: 'switch',
    },
    search: {
      type: 'select',
      select: {
        options: [
          { label: $gettext('Enabled'), value: 1 },
          { label: $gettext('Disabled'), value: 0 },
        ],
      },
    },
    sorter: true,
    pure: true,
  },
  {
    title: () => $gettext('Updated at'),
    dataIndex: 'updated_at',
    customRender: datetimeRender,
    sorter: true,
    pure: true,
  },
  {
    title: () => $gettext('Actions'),
    dataIndex: 'actions',
    fixed: 'right',
  },
]
</script>

<template>
  <StdCurd
    :title="$gettext('Deploy Targets')"
    :columns="columns"
    :api="certDeployTarget"
    disable-export
  >
    <template #beforeActions="{ record }">
      <AButton
        type="link"
        size="small"
        :loading="deployingStates[record.id] || false"
        @click="handleDeploy(record as CertDeployTarget)"
      >
        {{ $gettext('Deploy Now') }}
      </AButton>
      <AButton
        type="link"
        size="small"
        @click="showHistory(record as CertDeployTarget)"
      >
        {{ $gettext('History') }}
      </AButton>
    </template>
  </StdCurd>

  <DeployHistory
    v-model:open="historyOpen"
    :target="historyTarget"
  />
</template>
