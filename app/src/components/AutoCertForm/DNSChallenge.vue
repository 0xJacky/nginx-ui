<script setup lang="ts">
import type { SelectProps } from 'antdv-next'
import type { VNode } from 'vue'
import type { AutoCertOptions } from '@/api/auto_cert'
import type { DnsCredential } from '@/api/dns_credential'
import { PlusOutlined } from '@antdv-next/icons'
import { Button, Divider } from 'antdv-next'
import dns_credential from '@/api/dns_credential'
import { openDnsCredentialEditor } from '@/components/DnsCredentialEditor/openDnsCredentialEditor'

// Built-in credential picker, used when no plugin fills the DNS-01 slot of the
// certificate form.
const props = withDefaults(defineProps<{
  compact?: boolean
  /** Shows the current credential without letting it change. */
  readonly?: boolean
  /** Help text under a read-only picker. */
  readonlyHelp?: string
  /** Lists the credentials of the main node, which runs the challenge for the selected node. */
  mainNode?: boolean
}>(), {
  compact: false,
  readonly: false,
  readonlyHelp: '',
  mainNode: false,
})

const compactLabelCol = { flex: '170px' }
const compactWrapperCol = { flex: '1 1 0', style: { minWidth: 0 } }

interface DefaultOptionType {
  label?: string
  value?: string
}

const router = useRouter()

const data = defineModel<AutoCertOptions>('options', {
  required: true,
})

const loading = ref(false)
const loaded = ref(false)
const credentials = ref<DnsCredential[]>([])

function resolveProviderLabel(item: Pick<DnsCredential, 'provider' | 'provider_code' | 'code'>) {
  return item.provider || item.provider_code || item.code || $gettext('Unknown Provider')
}

const credentialOptions = computed<SelectProps['options']>(() => credentials.value.map(item => ({
  value: item.id,
  label: `${item.name} (${resolveProviderLabel(item)})`,
})))

interface CredentialMeta {
  id: number
  code: string
  provider?: string
  provider_code?: string
}

function applyCredentialMeta(item?: CredentialMeta) {
  if (!item) {
    data.value.dns_credential_id = undefined
    data.value.code = undefined
    data.value.provider = undefined
    data.value.provider_code = undefined
    return
  }

  data.value.dns_credential_id = item.id
  data.value.code = item.code
  data.value.provider = item.provider
  data.value.provider_code = item.provider_code || item.code
  // The dns01 plugin reads the selected credential from challenge_config.
  data.value.challenge_config = {
    ...data.value.challenge_config,
    credential_id: String(item.id),
  }
}

// An id of 0 means none is set, so the select shows its placeholder.
const selectedCredentialId = computed<SelectProps['value']>({
  get: () => data.value.dns_credential_id || undefined,
  set: value => {
    const selectedID = typeof value === 'number' || typeof value === 'string' ? Number(value) : undefined
    const current = selectedID === undefined || Number.isNaN(selectedID)
      ? undefined
      : credentials.value.find(item => item.id === selectedID)
    applyCredentialMeta(current)
  },
})

// A credential that was deleted after this certificate was set up.
const isMissingCredential = computed(() => loaded.value
  && !!data.value.dns_credential_id
  && !credentials.value.some(item => item.id === data.value.dns_credential_id))

async function loadCredentials() {
  loading.value = true
  try {
    const list: DnsCredential[] = []
    let page = 1

    while (true) {
      try {
        const r = await dns_credential.getList({ page }, props.mainNode ? { skipNodeProxy: true } : undefined)
        const rows = r?.data ?? []
        list.push(...rows)

        const perPage = r?.pagination?.per_page ?? 0
        if (!perPage || rows.length < perPage)
          break

        page++
      }
      catch {
        break
      }
    }

    credentials.value = list
    loaded.value = true

    const current = credentials.value.find(item => item.id === data.value.dns_credential_id)
    if (current && !props.readonly)
      applyCredentialMeta(current)
  }
  finally {
    loading.value = false
  }
}

function goToCredentialPage() {
  router.push('/dns/credentials')
}

async function createCredential() {
  const created = await openDnsCredentialEditor()
  if (!created)
    return

  await loadCredentials()
  applyCredentialMeta(credentials.value.find(item => item.id === created.id) ?? created)
}

function renderPopup(menu: VNode) {
  return h('div', [
    menu,
    h(Divider, { style: { margin: '4px 0' } }),
    h(Button, {
      type: 'link',
      size: 'small',
      icon: h(PlusOutlined),
      // Keep the focus in the select until the click lands.
      onMousedown: (e: MouseEvent) => e.preventDefault(),
      onClick: createCredential,
    }, () => $gettext('New credential')),
  ])
}

function filterOption(input: string, option?: DefaultOptionType) {
  const needle = input.toLowerCase()
  const label = option?.label?.toString().toLowerCase() ?? ''
  const value = option?.value?.toString().toLowerCase() ?? ''
  return label.includes(needle) || value.includes(needle)
}

const validateStatus = computed(() => isMissingCredential.value && !props.readonly ? 'error' : undefined)

const help = computed(() => {
  if (isMissingCredential.value)
    return $gettext('The credential was deleted. Please choose another one.')
  if (props.readonly)
    return props.readonlyHelp || undefined
  if (props.mainNode)
    return $gettext('Credentials of the main node. Switch to the main node to add or change one.')
  return undefined
})

// The select shows the raw id for a deleted credential; show a label instead.
const displayOptions = computed<SelectProps['options']>(() => {
  if (!isMissingCredential.value)
    return credentialOptions.value
  return [
    ...(credentialOptions.value ?? []),
    { value: data.value.dns_credential_id!, label: $gettext('Deleted credential'), disabled: true },
  ]
})

onMounted(async () => {
  await loadCredentials()
})
</script>

<template>
  <AForm
    :layout="props.compact ? 'horizontal' : 'vertical'"
    :label-align="props.compact ? 'left' : undefined"
    :label-col="props.compact ? compactLabelCol : undefined"
    :wrapper-col="props.compact ? compactWrapperCol : undefined"
    :model="data"
  >
    <AFormItem
      name="dns_credential_id"
      :label="$gettext('DNS Credential')"
      :required="!readonly"
      :validate-status="validateStatus"
      :help="help"
    >
      <ASpaceCompact block>
        <ASelect
          v-model:value="selectedCredentialId"
          class="min-w-0 flex-1"
          :options="displayOptions"
          :placeholder="$gettext('Select credential')"
          :loading="loading"
          :disabled="readonly"
          :popup-render="mainNode ? undefined : renderPopup"
          show-search
          :filter-option="filterOption"
        />
        <AButton v-if="!readonly && !mainNode" @click="goToCredentialPage">
          {{ $gettext('Manage') }}
        </AButton>
      </ASpaceCompact>
    </AFormItem>
  </AForm>
</template>
