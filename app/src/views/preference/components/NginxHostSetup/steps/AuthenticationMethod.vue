<script setup lang="ts">
import type { HostSetupParams, KeySource } from '../useHostSetupWizard'
import {
  CheckCircleOutlined,
  KeyOutlined,
  ReloadOutlined,
  SaveOutlined,
  UploadOutlined,
} from '@ant-design/icons-vue'
import { computed, onBeforeUnmount, onMounted, ref } from 'vue'
import hostSetup from '@/api/host_setup'
import settingsApi from '@/api/settings'
import { getErrorMessage } from '@/lib/http'
import { useLatestRequest } from '../useLatestRequest'

const params = defineModel<HostSetupParams>('params', { required: true })
const publicKey = defineModel<string>('publicKey', { default: '' })
const privateKeyOnce = defineModel<string>('privateKeyOnce', { default: '' })
const validatedPrivateKeyPath = defineModel<string>('validatedPrivateKeyPath', { default: '' })

const defaultPrivateKeyPath = '/etc/nginx-ui/host_key'
const maxPrivateKeySize = 64 * 1024
const keyError = ref('')
// Loading, regenerating and saving all answer the same question, so they share
// one error and invalidate each other.
const keyRequest = useLatestRequest({ error: keyError })
const saveRequest = useLatestRequest({ error: keyError })
const { isLoading: isLoadingKey } = keyRequest
const { isLoading: isSavingProvidedKey } = saveRequest
const privateKeyInput = ref('')
// False until a probe conclusively answered whether a key exists at the path.
// While false the regenerate action keeps its confirmation, because a key may
// still be sitting on disk.
const isKeyStateKnown = ref(false)
// A returning operator who already points at their own key must not find that
// configuration hidden, so the disclosure starts open in that case.
const ownKeyPanels = ref<string[]>([])
const { message } = useGlobalApp()

// The backend answers 404 only when no key exists at the path yet. Any other
// status is a real failure and must reach the operator.
function isKeyMissing(error: unknown): boolean {
  return (error as { response?: { status?: number } })?.response?.status === 404
}

const keySource = computed<KeySource>({
  get: () => params.value.key_source,
  set: value => {
    params.value.key_source = value
    privateKeyOnce.value = ''
    privateKeyInput.value = ''
    keyError.value = ''
    if (value !== 'existing') {
      params.value.container_key_path = defaultPrivateKeyPath
      params.value.host_key_path = ''
    }
    invalidateKey()
    if (value === 'generated')
      void loadPublicKey(false)
  },
})

const containerKeyPath = computed({
  get: () => params.value.container_key_path ?? '',
  set: value => {
    params.value.container_key_path = value
  },
})

const sourceHint = computed(() => {
  switch (keySource.value) {
    case 'existing':
      return $gettext('Point Nginx UI at a private key that already exists inside the container.')
    case 'provided':
      return $gettext('Paste or upload a private key. Nginx UI stores it at the managed path with mode 0600.')
    default:
      return $gettext('Nginx UI generates an ed25519 keypair and stores it at the managed path.')
  }
})

function invalidateKey() {
  keyRequest.invalidate()
  saveRequest.invalidate()
  validatedPrivateKeyPath.value = ''
  publicKey.value = ''
  isKeyStateKnown.value = false
}

async function loadPublicKey(showError = true) {
  const privateKeyPath = containerKeyPath.value.trim()
  invalidateKey()
  keyError.value = ''
  if (!privateKeyPath) {
    if (showError)
      keyError.value = $gettext('Private key path is required')
    return
  }

  // A missing key answers 404, which is an expected state on first run.
  await keyRequest.run(() => hostSetup.getPublicKey(privateKeyPath, { skipErrHandling: true }), {
    onSuccess: response => {
      if (privateKeyPath !== containerKeyPath.value.trim())
        return
      publicKey.value = response.public_key
      validatedPrivateKeyPath.value = privateKeyPath
      isKeyStateKnown.value = true
    },
    onError: error => {
      if (isKeyMissing(error)) {
        // A missing key is a normal starting state, not a failure to report.
        isKeyStateKnown.value = true
        // The 404 carries no message, so a dedicated text replaces the raw
        // transport error.
        if (showError)
          keyError.value = $gettext('No private key was found at this path')
        return
      }
      isKeyStateKnown.value = false
      keyError.value = getErrorMessage(error)
    },
  })
}

async function regenerate() {
  const privateKeyPath = containerKeyPath.value.trim() || defaultPrivateKeyPath
  params.value.container_key_path = privateKeyPath
  invalidateKey()
  keyError.value = ''
  await keyRequest.run(() => hostSetup.generateKeypair(privateKeyPath), {
    onSuccess: response => {
      publicKey.value = response.public_key
      privateKeyOnce.value = response.private_key ?? ''
      validatedPrivateKeyPath.value = privateKeyPath
      isKeyStateKnown.value = true
    },
  })
}

async function saveProvidedKey() {
  if (!privateKeyInput.value.trim()) {
    keyError.value = $gettext('Private key content is required')
    return
  }
  if (new Blob([privateKeyInput.value]).size > maxPrivateKeySize) {
    keyError.value = $gettext('Private key exceeds the 64 KiB limit')
    return
  }

  invalidateKey()
  keyError.value = ''
  await saveRequest.run(() => settingsApi.saveNginxPrivateKey(privateKeyInput.value), {
    onSuccess: response => {
      params.value.container_key_path = response.private_key_path
      params.value.key_source = 'provided'
      publicKey.value = response.public_key
      validatedPrivateKeyPath.value = response.private_key_path
      isKeyStateKnown.value = true
      privateKeyInput.value = ''
      message.success($gettext('Private key saved securely'))
    },
  })
}

// AUpload is used only as a file picker, so the request is always cancelled.
async function readPrivateKeyFile(file: File) {
  if (file.size > maxPrivateKeySize) {
    keyError.value = $gettext('Private key exceeds the 64 KiB limit')
    return false
  }
  try {
    privateKeyInput.value = await file.text()
    keyError.value = ''
  }
  catch (error) {
    keyError.value = getErrorMessage(error)
  }
  return false
}

onMounted(() => {
  if (keySource.value !== 'generated')
    ownKeyPanels.value = ['own-key']
  void loadPublicKey(false)
})

onBeforeUnmount(() => {
  privateKeyInput.value = ''
})
</script>

<template>
  <ACard size="small" :title="$gettext('Authentication')">
    <ACollapse v-model:active-key="ownKeyPanels" ghost class="mb-4">
      <ACollapsePanel key="own-key" :header="$gettext('Use my own SSH key')">
        <AFormItem :label="$gettext('Private key source')" required>
          <ASegmented
            v-model:value="keySource"
            block
            :options="[
              { label: $gettext('Generate'), value: 'generated' },
              { label: $gettext('Existing path'), value: 'existing' },
              { label: $gettext('Paste or upload'), value: 'provided' },
            ]"
          />
          <template #extra>
            <ATypographyText type="secondary" class="text-xs">
              {{ sourceHint }}
            </ATypographyText>
          </template>
        </AFormItem>

        <AAlert
          v-if="keySource === 'existing'"
          type="info"
          show-icon
          class="mb-4"
          :message="$gettext('The private key must already be available inside the Nginx UI container.')"
          :description="$gettext('Encrypted private keys are not supported. You may provide the host-side source path below to include a read-only bind mount in the generated container instructions.')"
        />

        <AAlert
          v-if="keySource === 'provided'"
          type="warning"
          show-icon
          class="mb-4"
          :message="$gettext('Saving will replace the Nginx UI managed SSH private key.')"
          :description="$gettext('Only unencrypted SSH private keys are accepted. The submitted content is cleared from the browser after it is stored with file mode 0600.')"
        />

        <AFormItem :label="$gettext('Private key path inside container')" required>
          <AInput
            v-model:value="containerKeyPath"
            :disabled="keySource !== 'existing'"
            placeholder="/etc/nginx-ui/host_key"
          />
          <template v-if="keySource !== 'existing'" #extra>
            <ATypographyText type="secondary" class="text-xs">
              {{ $gettext('Managed by Nginx UI. Switch to "Existing path" to use another location.') }}
            </ATypographyText>
          </template>
        </AFormItem>

        <AFormItem v-if="keySource === 'provided'" :label="$gettext('Private key content')" required>
          <ASpace direction="vertical" size="small" class="w-full">
            <AUpload
              :max-count="1"
              :show-upload-list="false"
              accept=".pem,.key,text/plain"
              :before-upload="readPrivateKeyFile"
            >
              <AButton>
                <UploadOutlined />
                {{ $gettext('Choose private key file') }}
              </AButton>
            </AUpload>
            <ATextarea
              v-model:value="privateKeyInput"
              :rows="8"
              :maxlength="maxPrivateKeySize"
              placeholder="-----BEGIN OPENSSH PRIVATE KEY-----"
            />
          </ASpace>
        </AFormItem>

        <AFormItem
          v-if="keySource === 'existing'"
          :label="$gettext('Private key path on Docker host (optional)')"
        >
          <AInput v-model:value="params.host_key_path" placeholder="/Users/name/.ssh/nginx_ui" />
          <template #extra>
            <ATypographyText type="secondary" class="text-xs">
              {{ $gettext('Adds a read-only bind mount to the generated container instructions.') }}
            </ATypographyText>
          </template>
        </AFormItem>
      </ACollapsePanel>
    </ACollapse>

    <AFormItem>
      <ASpace wrap>
        <template v-if="keySource === 'generated'">
          <APopconfirm
            v-if="publicKey || !isKeyStateKnown"
            :title="$gettext('Replace the stored private key with a new one?')"
            :description="$gettext('The current key stops working until the new public key is installed on the host.')"
            :ok-text="$gettext('Regenerate')"
            :cancel-text="$gettext('Cancel')"
            @confirm="regenerate"
          >
            <AButton :loading="isLoadingKey">
              <ReloadOutlined />
              {{ $gettext('Regenerate keypair') }}
            </AButton>
          </APopconfirm>
          <AButton v-else :loading="isLoadingKey" @click="regenerate">
            <KeyOutlined />
            {{ $gettext('Generate keypair') }}
          </AButton>
        </template>

        <AButton
          v-else-if="keySource === 'existing'"
          :loading="isLoadingKey"
          :disabled="!containerKeyPath.trim()"
          @click="loadPublicKey()"
        >
          <KeyOutlined />
          {{ $gettext('Validate existing key') }}
        </AButton>

        <AButton
          v-else
          type="primary"
          :loading="isSavingProvidedKey"
          :disabled="!privateKeyInput.trim()"
          @click="saveProvidedKey"
        >
          <SaveOutlined />
          {{ $gettext('Validate and save private key') }}
        </AButton>

        <ATag v-if="validatedPrivateKeyPath" color="success" :bordered="false">
          <CheckCircleOutlined />
          {{ $gettext('Key is readable') }}
        </ATag>
      </ASpace>
    </AFormItem>

    <AAlert
      v-if="keyError"
      type="error"
      show-icon
      class="mb-4"
      :message="$gettext('Private key validation failed')"
      :description="keyError"
    />

    <AFormItem :label="$gettext('Public key')">
      <ATypographyParagraph
        v-if="publicKey"
        class="mb-0 break-all"
        code
        :copyable="{ text: publicKey, tooltip: false }"
      >
        {{ publicKey }}
      </ATypographyParagraph>
      <ATypographyText v-else type="secondary">
        {{ $gettext('Generate a keypair to continue, or open "Use my own SSH key" above.') }}
      </ATypographyText>
      <template v-if="publicKey" #extra>
        <ATypographyText type="secondary" class="text-xs">
          {{ $gettext('The Install step shows where to put this on the host. Nothing has been installed there yet.') }}
        </ATypographyText>
      </template>
    </AFormItem>

    <AAlert
      v-if="privateKeyOnce"
      type="warning"
      show-icon
    >
      <template #message>
        {{ $gettext('Private key generated (shown once)') }}
      </template>
      <template #description>
        <p>{{ $gettext('Save this private key somewhere safe. It will NOT be shown again.') }}</p>
        <ATypographyParagraph
          class="mb-0 whitespace-pre-wrap break-all"
          code
          :copyable="{ text: privateKeyOnce, tooltip: false }"
        >
          {{ privateKeyOnce }}
        </ATypographyParagraph>
      </template>
    </AAlert>
  </ACard>
</template>
