<script setup lang="ts">
import type { Cert } from '@/api/cert'
import { DownOutlined } from '@antdv-next/icons'
import CertificateContentEditor from './CertificateContentEditor.vue'

const props = defineProps<{
  errors?: Record<string, string>
  /** Contents are written by Nginx UI, so they are shown read-only on demand. */
  managed: boolean
}>()

const data = defineModel<Cert>('data', { required: true })

const collapsed = ref(false)
const showContents = ref(!props.managed)

watch(() => props.managed, managed => {
  showContents.value = !managed
})

const hasPaths = computed(() => !!(data.value.ssl_certificate_path || data.value.ssl_certificate_key_path))

// Paths are typed in for other certificates, so a pasted certificate can be
// saved and a file already on disk can be referenced in place.
const editsPaths = computed(() => !props.managed)

function pathHelp(error: string | undefined, kind: string, wrongFile: string) {
  if (!error)
    return ''
  if (error.includes('required'))
    return $gettext('This field is required')
  return error.includes(kind) ? wrongFile : ''
}

const certPathHelp = computed(() => pathHelp(props.errors?.ssl_certificate_path, 'certificate_path', $gettext('The path exists, but the file is not a certificate')))
const keyPathHelp = computed(() => pathHelp(props.errors?.ssl_certificate_key_path, 'privatekey_path', $gettext('The path exists, but the file is not a private key')))
</script>

<template>
  <ACard size="small">
    <template #title>
      <button type="button" class="files-toggle" :aria-expanded="!collapsed" @click="collapsed = !collapsed">
        <DownOutlined class="files-chevron" :class="{ 'is-collapsed': collapsed }" />
        {{ $gettext('Certificate files') }}
      </button>
    </template>
    <template v-if="managed && !collapsed" #extra>
      <AButton type="link" size="small" class="px-0" @click="showContents = !showContents">
        {{ showContents ? $gettext('Hide contents') : $gettext('Show contents') }}
      </AButton>
    </template>

    <template v-if="!collapsed">
      <div v-if="editsPaths" class="files-inputs">
        <label class="files-field">
          <span class="files-label">{{ $gettext('SSL Certificate Path') }}</span>
          <AInput
            v-model:value="data.ssl_certificate_path"
            class="font-mono"
            :status="errors?.ssl_certificate_path ? 'error' : undefined"
          />
          <span v-if="certPathHelp" class="files-help">{{ certPathHelp }}</span>
        </label>
        <label class="files-field">
          <span class="files-label">{{ $gettext('SSL Certificate Key Path') }}</span>
          <AInput
            v-model:value="data.ssl_certificate_key_path"
            class="font-mono"
            :status="errors?.ssl_certificate_key_path ? 'error' : undefined"
          />
          <span v-if="keyPathHelp" class="files-help">{{ keyPathHelp }}</span>
        </label>
      </div>

      <dl v-else-if="hasPaths" class="files-paths">
        <dt>{{ $gettext('Certificate') }}</dt>
        <dd>
          <ATypographyText :copyable="!!data.ssl_certificate_path" class="font-mono">
            {{ data.ssl_certificate_path || '-' }}
          </ATypographyText>
        </dd>
        <dt>{{ $gettext('Private key') }}</dt>
        <dd>
          <ATypographyText :copyable="!!data.ssl_certificate_key_path" class="font-mono">
            {{ data.ssl_certificate_key_path || '-' }}
          </ATypographyText>
        </dd>
      </dl>

      <CertificateContentEditor
        v-if="showContents"
        v-model:data="data"
        :class="{ 'mt-3': editsPaths || hasPaths }"
        :errors="errors"
        :readonly="managed"
      />
    </template>
  </ACard>
</template>

<style scoped lang="less">
.files-toggle {
  display: inline-flex;
  align-items: center;
  gap: 6px;
  padding: 0;
  border: 0;
  background: none;
  color: inherit;
  font: inherit;
  cursor: pointer;
}

.files-chevron {
  font-size: 11px;
  transition: transform 0.2s;

  &.is-collapsed {
    transform: rotate(-90deg);
  }
}

.files-inputs {
  display: grid;
  grid-template-columns: repeat(auto-fit, minmax(240px, 1fr));
  gap: 12px 16px;
}

.files-field {
  display: flex;
  flex-direction: column;
  gap: 4px;
  min-width: 0;
}

.files-label {
  color: var(--ant-color-text-secondary);
}

.files-help {
  color: var(--ant-color-error);
}

.files-paths {
  display: grid;
  grid-template-columns: max-content minmax(0, 1fr);
  gap: 6px 24px;
  margin: 0;

  dt {
    color: var(--ant-color-text-secondary);
  }

  dd {
    margin: 0;
    min-width: 0;
    overflow-wrap: anywhere;
  }
}
</style>
