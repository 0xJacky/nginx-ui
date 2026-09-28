<script setup lang="ts">
import type { SelectProps } from 'antdv-next'
import type { ManagedUpstream, ManagedUpstreamServer, UpstreamMethod, UpstreamReference } from '@/api/upstream'
import { DeleteOutlined, PlusOutlined } from '@antdv-next/icons'
import { watchDebounced } from '@vueuse/core'
import upstream from '@/api/upstream'
import CodeEditor from '@/components/CodeEditor'
import { translateError } from '@/lib/http/error'
import { normalizeHttpError } from '@/lib/http/normalizeError'
import { DEFAULT_ZONE_SIZE_KB, kbToZoneSize, MIN_ZONE_SIZE_KB, zoneSizeToKb } from '../zoneSize'

const props = defineProps<{
  // Name of the upstream to edit; empty to create a new one.
  name?: string
}>()

const emit = defineEmits<{
  saved: [name: string]
}>()

const open = defineModel<boolean>('open', { default: false })

const { message } = App.useApp()

// The select needs a non-empty value for the default method.
type FormMethod = Exclude<UpstreamMethod, ''> | 'round_robin'

interface FormState extends Omit<ManagedUpstream, 'method' | 'zone_size'> {
  method: FormMethod
  zoneSizeKb: number | null
}

function createServer(): ManagedUpstreamServer {
  return { address: '', weight: null, max_fails: null, fail_timeout: '', backup: false, down: false }
}

function createForm(): FormState {
  return {
    name: '',
    method: 'round_robin',
    hash_key: '',
    consistent: false,
    keepalive: 0,
    // New groups share their balancing state across worker processes.
    zone: true,
    zoneSizeKb: DEFAULT_ZONE_SIZE_KB,
    servers: [createServer()],
    extra_directives: '',
  }
}

const form = reactive<FormState>(createForm())
const references = ref<UpstreamReference[]>([])
const filePath = ref('')
const isLoading = ref(false)
const isSaving = ref(false)
const previewContent = ref('')
const previewError = ref('')

const isEditing = computed(() => !!props.name)

const extraDirectivesPlaceholder = 'keepalive_timeout 60s;\nkeepalive_requests 1000;'

const methodOptions = computed<SelectProps['options']>(() => [
  { label: $gettext('Round Robin (default)'), value: 'round_robin' },
  { label: $gettext('Least Connections (least_conn)'), value: 'least_conn' },
  { label: $gettext('IP Hash (ip_hash)'), value: 'ip_hash' },
  { label: $gettext('Hash (hash)'), value: 'hash' },
  { label: $gettext('Random (random)'), value: 'random' },
])

// nginx rejects `backup` together with these methods.
const isBackupSupported = computed(() => !['hash', 'ip_hash', 'random'].includes(form.method))

function toPayload(): ManagedUpstream {
  const { zoneSizeKb, ...rest } = form
  return {
    ...rest,
    name: props.name || form.name.trim(),
    method: form.method === 'round_robin' ? '' : form.method,
    zone_size: form.zone ? kbToZoneSize(zoneSizeKb) : undefined,
    servers: form.servers.map(server => ({
      ...server,
      backup: isBackupSupported.value && server.backup,
      weight: server.weight ?? undefined,
      max_fails: server.max_fails ?? undefined,
    })),
  }
}

async function load() {
  Object.assign(form, createForm())
  references.value = []
  filePath.value = ''
  previewContent.value = ''
  previewError.value = ''

  if (!props.name)
    return

  isLoading.value = true
  try {
    const detail = await upstream.getManaged(props.name)
    Object.assign(form, {
      name: detail.name,
      method: detail.method || 'round_robin',
      hash_key: detail.hash_key ?? '',
      consistent: detail.consistent,
      keepalive: detail.keepalive,
      // Groups saved without a zone keep it off until the operator opts in.
      zone: detail.zone,
      zoneSizeKb: zoneSizeToKb(detail.zone_size) ?? DEFAULT_ZONE_SIZE_KB,
      servers: detail.servers.length > 0 ? detail.servers.map(server => ({ ...createServer(), ...server })) : [createServer()],
      extra_directives: detail.extra_directives ?? '',
    })
    references.value = detail.references
    filePath.value = detail.path
    previewContent.value = detail.content
  }
  catch {
    open.value = false
  }
  finally {
    isLoading.value = false
  }
}

watch(open, value => {
  if (value)
    load()
}, { immediate: true })

async function refreshPreview() {
  if (!open.value || isLoading.value)
    return
  const payload = toPayload()
  // Nothing useful to validate until the new upstream has a name.
  if (!payload.name) {
    previewContent.value = ''
    previewError.value = ''
    return
  }
  try {
    const res = await upstream.previewManaged(payload)
    previewContent.value = res.content
    previewError.value = ''
  }
  catch (error) {
    previewError.value = await translateError(normalizeHttpError(error))
  }
}

watchDebounced(form, refreshPreview, { debounce: 300, deep: true })

function addServer() {
  form.servers.push(createServer())
}

function removeServer(index: number) {
  form.servers.splice(index, 1)
}

async function save() {
  isSaving.value = true
  try {
    const payload = toPayload()
    const detail = isEditing.value
      ? await upstream.updateManaged(payload.name, payload)
      : await upstream.createManaged(payload)
    message.success($gettext('Upstream %{name} saved and Nginx reloaded', { name: detail.name }))
    emit('saved', detail.name)
    open.value = false
  }
  catch {
    // The request layer already shows the error, including nginx -t output.
  }
  finally {
    isSaving.value = false
  }
}
</script>

<template>
  <ADrawer
    v-model:open="open"
    :title="isEditing ? $gettext('Edit Upstream %{name}', { name: props.name ?? '' }) : $gettext('Create Upstream')"
    :size="760"
    destroy-on-hidden
    data-testid="upstream-editor"
  >
    <ASpin :spinning="isLoading">
      <AForm layout="vertical">
        <AFormItem
          :label="$gettext('Name')"
          required
          :extra="$gettext('Sites use it as proxy_pass http://%{name};', { name: form.name || 'name' })"
        >
          <AInput
            v-model:value="form.name"
            class="max-w-100"
            :disabled="isEditing"
            placeholder="backend_pool"
            data-testid="upstream-name"
          />
        </AFormItem>

        <AFormItem :label="$gettext('Load Balancing Method')">
          <ASelect
            v-model:value="form.method"
            class="w-full max-w-100"
            :options="methodOptions"
            data-testid="upstream-method"
          />
        </AFormItem>

        <AFlex
          v-if="form.method === 'hash'"
          gap="middle"
          wrap
          align="end"
        >
          <AFormItem
            :label="$gettext('Hash Key')"
            required
            class="min-w-60 flex-1"
          >
            <AInput
              v-model:value="form.hash_key"
              placeholder="$request_uri"
            />
          </AFormItem>
          <AFormItem :label="$gettext('Consistent Hashing')">
            <ASwitch v-model:checked="form.consistent" />
          </AFormItem>
        </AFlex>

        <AFormItem
          :label="$gettext('Servers')"
          required
        >
          <div class="flex flex-col gap-3">
            <div class="hidden md:grid server-grid gap-2 text-xs text-gray-500 dark:text-gray-400">
              <span>{{ $gettext('Address') }}</span>
              <span>{{ $gettext('Weight') }}</span>
              <span>{{ $gettext('Max Fails') }}</span>
              <span>{{ $gettext('Fail Timeout') }}</span>
              <span>{{ $gettext('Backup') }}</span>
              <span>{{ $gettext('Enabled') }}</span>
              <span />
            </div>
            <div
              v-for="(server, index) in form.servers"
              :key="index"
              class="grid server-grid gap-2 items-center"
              data-testid="upstream-server-row"
            >
              <AInput
                v-model:value="server.address"
                placeholder="127.0.0.1:8080"
                :aria-label="$gettext('Address')"
                data-testid="upstream-server-address"
              />
              <AInputNumber
                v-model:value="server.weight"
                class="w-full"
                :min="1"
                placeholder="1"
                :aria-label="$gettext('Weight')"
              />
              <AInputNumber
                v-model:value="server.max_fails"
                class="w-full"
                :min="0"
                placeholder="1"
                :aria-label="$gettext('Max Fails')"
              />
              <AInput
                v-model:value="server.fail_timeout"
                placeholder="10s"
                :aria-label="$gettext('Fail Timeout')"
              />
              <ATooltip :title="isBackupSupported ? '' : $gettext('Not available with this load balancing method')">
                <ACheckbox
                  v-model:checked="server.backup"
                  :disabled="!isBackupSupported"
                  :aria-label="$gettext('Backup')"
                >
                  <span class="md:hidden">{{ $gettext('Backup') }}</span>
                </ACheckbox>
              </ATooltip>
              <ASwitch
                :checked="!server.down"
                :aria-label="$gettext('Enabled')"
                data-testid="upstream-server-enabled"
                @change="checked => server.down = !checked"
              />
              <AButton
                type="text"
                danger
                :disabled="form.servers.length <= 1"
                :aria-label="$gettext('Remove')"
                @click="removeServer(index)"
              >
                <template #icon>
                  <DeleteOutlined />
                </template>
              </AButton>
            </div>
            <AButton
              block
              type="dashed"
              data-testid="upstream-add-server"
              @click="addServer"
            >
              <template #icon>
                <PlusOutlined />
              </template>
              {{ $gettext('Add Server') }}
            </AButton>
          </div>
        </AFormItem>

        <AFormItem
          :label="$gettext('Shared Memory Zone')"
          :extra="$gettext('Keeps the group\'s state in shared memory as zone %{name}, so every worker process balances across the same servers and shares their failure counts. Without it each worker balances on its own.', { name: form.name || 'name' })"
        >
          <AFlex
            gap="middle"
            align="center"
            wrap
          >
            <ASwitch
              v-model:checked="form.zone"
              :aria-label="$gettext('Shared Memory Zone')"
              data-testid="upstream-zone-enabled"
            />
            <ASpaceCompact v-if="form.zone">
              <AInputNumber
                v-model:value="form.zoneSizeKb"
                class="w-30"
                :min="MIN_ZONE_SIZE_KB"
                :precision="0"
                :aria-label="$gettext('Zone Size')"
                data-testid="upstream-zone-size"
              />
              <ASpaceAddon>KB</ASpaceAddon>
            </ASpaceCompact>
          </AFlex>
        </AFormItem>

        <AFormItem
          :label="$gettext('Keepalive')"
          :extra="$gettext('Idle connections to keep open per worker; 0 disables it. Proxied sites also need proxy_http_version 1.1 and an empty Connection header.')"
        >
          <ASpaceCompact>
            <AInputNumber
              v-model:value="form.keepalive"
              class="w-30"
              :min="0"
            />
            <ASpaceAddon>{{ $gettext('Connections') }}</ASpaceAddon>
          </ASpaceCompact>
        </AFormItem>

        <AFormItem
          :label="$gettext('Additional Directives')"
          :extra="$gettext('One directive per line, for example keepalive_timeout or keepalive_requests.')"
        >
          <ATextarea
            v-model:value="form.extra_directives"
            class="font-mono"
            :auto-size="{ minRows: 2, maxRows: 6 }"
            :placeholder="extraDirectivesPlaceholder"
          />
        </AFormItem>

        <AFormItem
          v-if="isEditing"
          :label="$gettext('Referenced By')"
        >
          <div
            v-if="references.length > 0"
            class="flex flex-wrap gap-2"
          >
            <ATag
              v-for="reference in references"
              :key="reference.path"
              color="blue"
            >
              {{ reference.name }}
            </ATag>
          </div>
          <span
            v-else
            class="text-gray-500 dark:text-gray-400"
          >
            {{ $gettext('No site uses this upstream yet.') }}
          </span>
        </AFormItem>

        <AFormItem
          :label="$gettext('Configuration Preview')"
          :extra="filePath"
        >
          <AAlert
            v-if="previewError"
            type="warning"
            show-icon
            class="mb-2"
            :title="previewError"
            data-testid="upstream-preview-error"
          />
          <CodeEditor
            :content="previewContent"
            readonly
            default-height="180px"
          />
        </AFormItem>
      </AForm>
    </ASpin>

    <template #footer>
      <AFlex justify="end" gap="small">
        <AButton @click="open = false">
          {{ $gettext('Cancel') }}
        </AButton>
        <AButton
          type="primary"
          :loading="isSaving"
          data-testid="upstream-save"
          @click="save"
        >
          {{ $gettext('Save') }}
        </AButton>
      </AFlex>
    </template>
  </ADrawer>
</template>

<style scoped>
.server-grid {
  grid-template-columns: 1fr;
}

@media (min-width: 768px) {
  .server-grid {
    grid-template-columns: minmax(0, 1fr) 76px 76px 76px 64px 56px 32px;
  }
}
</style>
