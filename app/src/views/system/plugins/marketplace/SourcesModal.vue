<script setup lang="ts">
import { DeleteOutlined, PlusOutlined } from '@antdv-next/icons'
import { getMarketplaceSources, saveMarketplaceSources } from '@/api/plugin_marketplace'
import { getErrorMessage } from '@/lib/http'

const emit = defineEmits<{
  saved: []
}>()

const open = defineModel<boolean>('open', { default: false })

const { message } = App.useApp()

const loading = ref(false)
const saving = ref(false)
const error = ref('')
const sources = ref<string[]>([])
const defaultSource = ref('')

const canRestoreDefault = computed(() => Boolean(defaultSource.value) && !sources.value.includes(defaultSource.value))

async function load() {
  loading.value = true
  error.value = ''
  try {
    const response = await getMarketplaceSources()
    sources.value = [...response.sources]
    defaultSource.value = response.default
  }
  catch (e) {
    error.value = getErrorMessage(e, $gettext('Failed to load the marketplace sources'))
  }
  finally {
    loading.value = false
  }
}

function addSource() {
  sources.value.push('')
}

function removeSource(index: number) {
  sources.value.splice(index, 1)
}

function restoreDefault() {
  sources.value.push(defaultSource.value)
}

async function save() {
  saving.value = true
  error.value = ''
  try {
    const response = await saveMarketplaceSources(sources.value.map(item => item.trim()).filter(Boolean))
    sources.value = [...response.sources]
    message.success($gettext('Marketplace sources saved'))
    open.value = false
    emit('saved')
  }
  catch (e) {
    error.value = getErrorMessage(e, $gettext('Failed to save the marketplace sources'))
  }
  finally {
    saving.value = false
  }
}

watch(open, value => {
  if (value)
    load()
})
</script>

<template>
  <AModal
    v-model:open="open"
    :title="$gettext('Marketplace sources')"
    :width="620"
    :ok-text="$gettext('Save')"
    :cancel-text="$gettext('Cancel')"
    :confirm-loading="saving"
    @ok="save"
  >
    <ASpin :spinning="loading">
      <AAlert
        v-if="error"
        type="error"
        show-icon
        class="mb-4"
        :title="error"
      />

      <p class="text-gray-500">
        {{ $gettext('Catalogs are merged in order, the first source that offers a plugin wins. Leave the list empty to use the official catalog.') }}
      </p>

      <div v-for="(_, index) in sources" :key="index" class="mb-2 flex items-center gap-2">
        <AInput
          v-model:value="sources[index]"
          placeholder="https://example.com/plugins/index.json"
          allow-clear
        />
        <AButton danger type="text" @click="removeSource(index)">
          <template #icon>
            <DeleteOutlined />
          </template>
        </AButton>
      </div>

      <AEmpty v-if="sources.length === 0" :description="$gettext('Using the official catalog')" />

      <ASpace wrap class="mt-2">
        <AButton @click="addSource">
          <template #icon>
            <PlusOutlined />
          </template>
          {{ $gettext('Add source') }}
        </AButton>
        <AButton v-if="canRestoreDefault" type="link" @click="restoreDefault">
          {{ $gettext('Add the official catalog') }}
        </AButton>
      </ASpace>
    </ASpin>
  </AModal>
</template>
