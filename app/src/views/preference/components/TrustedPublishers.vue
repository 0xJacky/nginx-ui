<script setup lang="ts">
import { DeleteOutlined, PlusOutlined } from '@antdv-next/icons'

// Each entry is a public key line optionally followed by a publisher name,
// the format the backend stores in plugin.trusted_public_keys.
const entries = defineModel<string[]>({ required: true })

interface Publisher {
  entry: string
  key: string
  name: string
  id: string
}

// A public key line is 42 bytes in base64: the algorithm, the id and the key.
const KEY_PATTERN = /^[A-Z0-9+/]{56}$/i

function parseEntry(entry: string) {
  const line = entry
    .split('\n')
    .map(item => item.trim())
    .find(item => item && !item.startsWith('untrusted comment:')) ?? ''
  const index = line.search(/\s/)
  if (index < 0)
    return { key: line, name: '' }
  return { key: line.slice(0, index), name: line.slice(index).trim() }
}

// The id matches the one the backend prints: the little endian id bytes as
// upper case hex.
function keyID(key: string) {
  if (!KEY_PATTERN.test(key))
    return ''
  try {
    const bytes = Uint8Array.from(atob(key), char => char.charCodeAt(0))
    let id = 0n
    for (let i = 9; i >= 2; i--)
      id = (id << 8n) | BigInt(bytes[i])
    return id.toString(16).toUpperCase().padStart(16, '0')
  }
  catch {
    return ''
  }
}

const publishers = computed<Publisher[]>(() => entries.value.map(entry => {
  const { key, name } = parseEntry(entry)
  return { entry, key, name, id: keyID(key) }
}))

function remove(index: number) {
  entries.value = entries.value.filter((_, i) => i !== index)
}

const isModalOpen = ref(false)
const form = reactive({ name: '', key: '' })
const keyError = ref('')

function openModal() {
  form.name = ''
  form.key = ''
  keyError.value = ''
  isModalOpen.value = true
}

function add() {
  // A pasted key file works too, its comment line is dropped.
  const { key } = parseEntry(form.key)
  const id = keyID(key)
  if (!id) {
    keyError.value = $gettext('This is not a valid public key. Paste the whole key the publisher gave you.')
    return
  }
  if (publishers.value.some(item => item.id === id)) {
    keyError.value = $gettext('This publisher is already in the list.')
    return
  }
  // The name is stored on the same line, so it cannot span lines.
  const name = form.name.replace(/\s+/g, ' ').trim()
  entries.value = [...entries.value, name ? `${key} ${name}` : key]
  isModalOpen.value = false
}
</script>

<template>
  <div>
    <div v-if="!publishers.length" class="text-sm text-gray-500">
      {{ $gettext('No publishers yet.') }}
    </div>
    <div v-else class="publisher-list">
      <div
        v-for="(item, index) in publishers"
        :key="item.entry"
        class="publisher-item"
      >
        <div class="min-w-0">
          <div class="publisher-name">
            {{ item.name || $gettext('Unnamed publisher') }}
          </div>
          <div class="publisher-id">
            {{ item.id ? `ID ${item.id}` : $gettext('Not a valid public key') }}
          </div>
        </div>
        <APopconfirm
          :title="$gettext('Are you sure you want to remove this item?')"
          :ok-text="$gettext('Yes')"
          :cancel-text="$gettext('No')"
          @confirm="remove(index)"
        >
          <AButton type="text" danger size="small">
            <template #icon>
              <DeleteOutlined />
            </template>
          </AButton>
        </APopconfirm>
      </div>
    </div>

    <AButton class="mt-3" size="small" @click="openModal">
      <template #icon>
        <PlusOutlined />
      </template>
      {{ $gettext('Add publisher') }}
    </AButton>

    <AModal
      v-model:open="isModalOpen"
      :title="$gettext('Add publisher')"
      :ok-text="$gettext('Add')"
      destroy-on-hidden
      @ok="add"
    >
      <AForm layout="vertical">
        <AFormItem :label="$gettext('Name')">
          <AInput
            v-model:value="form.name"
            :placeholder="$gettext('Shown in this list only')"
            :maxlength="64"
          />
        </AFormItem>
        <AFormItem
          :label="$gettext('Public Key')"
          :validate-status="keyError ? 'error' : undefined"
          :help="keyError || undefined"
          required
        >
          <ATextarea
            v-model:value="form.key"
            :rows="3"
            :placeholder="$gettext('Paste the public key the publisher gave you')"
            class="font-mono"
            @change="keyError = ''"
          />
        </AFormItem>
      </AForm>
    </AModal>
  </div>
</template>

<style lang="less" scoped>
.publisher-list {
  border: 1px solid var(--ant-color-border-secondary);
  border-radius: 8px;
}

.publisher-item {
  display: flex;
  align-items: center;
  justify-content: space-between;
  gap: 12px;
  padding: 8px 12px;

  & + & {
    border-top: 1px solid var(--ant-color-split);
  }
}

.publisher-name {
  overflow: hidden;
  text-overflow: ellipsis;
  white-space: nowrap;
}

.publisher-id {
  font-family: var(--ant-font-family-code, monospace);
  font-size: 12px;
  color: var(--ant-color-text-tertiary);
}
</style>
