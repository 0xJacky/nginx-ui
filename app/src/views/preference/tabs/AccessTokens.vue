<script setup lang="ts">
import type { Dayjs } from 'dayjs'
import type { ServiceToken, ServiceTokenScope } from '@/api/service_token'
import { CopyOutlined, DeleteOutlined, KeyOutlined, PlusOutlined, SyncOutlined } from '@ant-design/icons-vue'
import { useClipboard } from '@vueuse/core'
import dayjs from 'dayjs'
import relativeTime from 'dayjs/plugin/relativeTime'
import serviceToken from '@/api/service_token'
import { formatDateTime } from '@/lib/helper'

dayjs.extend(relativeTime)

const { message } = App.useApp()
const { copy, isSupported: isClipboardSupported } = useClipboard()

const isLoading = ref(false)
const isCreating = ref(false)
const isCreateModalOpen = ref(false)
const isSecretModalOpen = ref(false)
const tokens = ref<ServiceToken[]>([])
const tokenName = ref('')
const selectedScopes = ref<ServiceTokenScope[]>(['api:read'])
const expiresAt = ref<Dayjs>()
const oneTimeToken = ref('')

const scopeOptions: Array<{ value: ServiceTokenScope, label: string, description: string }> = [
  {
    value: 'api:read',
    label: $gettext('Management API: Read'),
    description: $gettext('Read resources and runtime status through the management API.'),
  },
  {
    value: 'api:write',
    label: $gettext('Management API: Write'),
    description: $gettext('Create, update, and delete resources and run Nginx control operations. Includes API read access.'),
  },
  {
    value: 'mcp:read',
    label: $gettext('MCP: Read'),
    description: $gettext('Use read-only Nginx UI MCP tools.'),
  },
  {
    value: 'mcp:write',
    label: $gettext('MCP: Write'),
    description: $gettext('Use mutating Nginx UI MCP tools. Includes MCP read access.'),
  },
]

const columns = [
  { title: $gettext('Name'), dataIndex: 'name' },
  { title: $gettext('Scopes'), dataIndex: 'scopes' },
  { title: $gettext('Last used'), dataIndex: 'last_used_at' },
  { title: $gettext('Expires'), dataIndex: 'expires_at' },
  { title: $gettext('Status'), dataIndex: 'status' },
  { title: $gettext('Action'), dataIndex: 'action', width: 120 },
]

async function loadTokens() {
  isLoading.value = true
  try {
    const data = await serviceToken.list()
    tokens.value = data
  }
  finally {
    isLoading.value = false
  }
}

function resetCreateForm() {
  tokenName.value = ''
  selectedScopes.value = ['api:read']
  expiresAt.value = undefined
}

function openCreateModal() {
  resetCreateForm()
  isCreateModalOpen.value = true
}

function showOneTimeToken(token?: string) {
  if (!token)
    return
  oneTimeToken.value = token
  isSecretModalOpen.value = true
}

async function createToken() {
  if (!tokenName.value.trim()) {
    message.error($gettext('Token name is required'))
    return
  }
  if (selectedScopes.value.length === 0) {
    message.error($gettext('Select at least one scope'))
    return
  }
  isCreating.value = true
  try {
    const token = await serviceToken.create({
      name: tokenName.value.trim(),
      scopes: selectedScopes.value,
      expires_at: expiresAt.value?.toISOString(),
    })
    isCreateModalOpen.value = false
    showOneTimeToken(token.token)
    await loadTokens()
    message.success($gettext('Access token created'))
  }
  finally {
    isCreating.value = false
  }
}

async function rotateToken(record: Record<string, unknown>) {
  const token = await serviceToken.rotate(String(record.id))
  showOneTimeToken(token.token)
  await loadTokens()
  message.success($gettext('Access token rotated'))
}

async function revokeToken(record: Record<string, unknown>) {
  await serviceToken.revoke(String(record.id))
  await loadTokens()
  message.success($gettext('Access token revoked'))
}

async function copyToken() {
  try {
    await copy(oneTimeToken.value)
    message.success($gettext('Access token copied to clipboard'))
  }
  catch {
    message.error($gettext('Failed to copy access token'))
  }
}

function closeSecretModal() {
  oneTimeToken.value = ''
  isSecretModalOpen.value = false
}

function isTokenExpired(record: Record<string, unknown>) {
  return Boolean(record.expires_at && dayjs(String(record.expires_at)).isBefore(dayjs()))
}

function tokenStatus(record: Record<string, unknown>) {
  if (record.revoked_at)
    return { color: 'default', text: $gettext('Revoked') }
  if (isTokenExpired(record))
    return { color: 'error', text: $gettext('Expired') }
  return { color: 'success', text: $gettext('Active') }
}

function scopeLabel(scope: ServiceTokenScope) {
  return scopeOptions.find(option => option.value === scope)?.label ?? scope
}

onMounted(loadTokens)
</script>

<template>
  <div>
    <div class="mb-5 flex flex-wrap items-start justify-between gap-4">
      <div class="max-w-2xl">
        <h3 class="mb-1">
          {{ $gettext('Access Tokens') }}
        </h3>
        <p class="mb-0 text-gray-500">
          {{ $gettext('Create scoped credentials for the Nginx UI CLI, automation, and MCP clients. Tokens are shown only once.') }}
        </p>
      </div>
      <AButton type="primary" @click="openCreateModal">
        <template #icon>
          <PlusOutlined />
        </template>
        {{ $gettext('Create Token') }}
      </AButton>
    </div>

    <AAlert
      class="mb-4"
      show-icon
      type="info"
      :message="$gettext('Use the smallest required scope and set an expiration date for automation credentials.')"
    />

    <ATable
      :columns="columns"
      :data-source="tokens"
      :loading="isLoading"
      row-key="id"
      size="small"
      :pagination="false"
      :scroll="{ x: 760 }"
    >
      <template #emptyText>
        <AEmpty :description="$gettext('No access tokens')" />
      </template>
      <template #bodyCell="{ column, record }">
        <template v-if="column.dataIndex === 'name'">
          <div class="flex items-center gap-2">
            <KeyOutlined class="text-gray-400" />
            <div>
              <div class="font-medium">
                {{ record.name }}
              </div>
              <div class="font-mono text-xs text-gray-400">
                {{ record.id }}
              </div>
            </div>
          </div>
        </template>
        <template v-else-if="column.dataIndex === 'scopes'">
          <div class="flex flex-wrap gap-1">
            <ATag v-for="scope in record.scopes" :key="scope">
              {{ scopeLabel(scope) }}
            </ATag>
          </div>
        </template>
        <template v-else-if="column.dataIndex === 'last_used_at'">
          <ATooltip v-if="record.last_used_at" :title="formatDateTime(record.last_used_at)">
            {{ dayjs(record.last_used_at).fromNow() }}
          </ATooltip>
          <span v-else class="text-gray-400">{{ $gettext('Never') }}</span>
        </template>
        <template v-else-if="column.dataIndex === 'expires_at'">
          <span v-if="record.expires_at">{{ formatDateTime(record.expires_at) }}</span>
          <span v-else class="text-gray-400">{{ $gettext('Never') }}</span>
        </template>
        <template v-else-if="column.dataIndex === 'status'">
          <ATag :color="tokenStatus(record).color">
            {{ tokenStatus(record).text }}
          </ATag>
        </template>
        <template v-else-if="column.dataIndex === 'action'">
          <div v-if="!record.revoked_at" class="flex">
            <APopconfirm
              v-if="!isTokenExpired(record)"
              :title="$gettext('Rotate this token? The current token will stop working immediately.')"
              @confirm="rotateToken(record)"
            >
              <AButton type="link" size="small" :aria-label="$gettext('Rotate token')">
                <SyncOutlined />
              </AButton>
            </APopconfirm>
            <APopconfirm
              :title="$gettext('Revoke this token? This action cannot be undone.')"
              @confirm="revokeToken(record)"
            >
              <AButton type="link" danger size="small" :aria-label="$gettext('Revoke token')">
                <DeleteOutlined />
              </AButton>
            </APopconfirm>
          </div>
        </template>
      </template>
    </ATable>

    <AModal
      v-model:open="isCreateModalOpen"
      :title="$gettext('Create Access Token')"
      :confirm-loading="isCreating"
      :ok-text="$gettext('Create')"
      @ok="createToken"
    >
      <AForm layout="vertical" class="pt-2">
        <AFormItem :label="$gettext('Name')" required>
          <AInput
            v-model:value="tokenName"
            :maxlength="64"
            :placeholder="$gettext('For example: production deployment')"
            @press-enter="createToken"
          />
        </AFormItem>
        <AFormItem :label="$gettext('Scopes')" required>
          <ACheckboxGroup v-model:value="selectedScopes" class="w-full">
            <div class="grid gap-2">
              <label
                v-for="option in scopeOptions"
                :key="option.value"
                class="scope-option"
              >
                <ACheckbox :value="option.value" />
                <span>
                  <span class="block font-medium">{{ option.label }}</span>
                  <span class="block text-xs text-gray-500">{{ option.description }}</span>
                </span>
              </label>
            </div>
          </ACheckboxGroup>
        </AFormItem>
        <AFormItem :label="$gettext('Expires At')">
          <ADatePicker
            v-model:value="expiresAt"
            class="w-full"
            show-time
            :disabled-date="current => current && current.isBefore(dayjs().startOf('day'))"
            :placeholder="$gettext('No expiration')"
          />
        </AFormItem>
      </AForm>
    </AModal>

    <AModal
      :open="isSecretModalOpen"
      :title="$gettext('Save Your Access Token')"
      :closable="false"
      :mask-closable="false"
    >
      <AAlert
        class="mb-4"
        show-icon
        type="warning"
        :message="$gettext('This token is shown only once. Store it in a secure secret manager before closing this dialog.')"
      />
      <AInputGroup compact class="flex">
        <AInput :value="oneTimeToken" readonly class="min-w-0 flex-1 font-mono" />
        <AButton :disabled="!isClipboardSupported" @click="copyToken">
          <CopyOutlined />
          {{ $gettext('Copy') }}
        </AButton>
      </AInputGroup>
      <template #footer>
        <AButton type="primary" @click="closeSecretModal">
          {{ $gettext('I have saved the token') }}
        </AButton>
      </template>
    </AModal>
  </div>
</template>

<style scoped lang="less">
.scope-option {
  display: flex;
  gap: 10px;
  align-items: flex-start;
  padding: 10px 12px;
  border: 1px solid var(--ant-color-border, #d9d9d9);
  border-radius: 6px;
  cursor: pointer;
}

.scope-option:hover {
  border-color: var(--ant-color-primary, #1677ff);
}
</style>
