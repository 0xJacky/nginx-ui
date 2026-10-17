<script setup lang="ts">
import type { BuiltinTemplate } from '@/api/snippet'
import { CopyOutlined } from '@antdv-next/icons'
import snippet from '@/api/snippet'
import BaseEditor from '@/components/BaseEditor'
import FooterToolBar from '@/components/FooterToolbar'
import SnippetCode from './components/SnippetCode.vue'
import SnippetPreview from './components/SnippetPreview.vue'
import VariableSummary from './components/VariableSummary.vue'
import { useSnippetDescription } from './description'
import { toRows } from './variables'

const route = useRoute()
const router = useRouter()
const { describe } = useSnippetDescription()

const name = computed(() => String(route.params.name))
const pluginId = computed(() => typeof route.query.plugin_id === 'string' ? route.query.plugin_id : undefined)
const source = ref<BuiltinTemplate>()
const content = ref('')
const isLoading = ref(false)

const rows = computed(() => toRows(source.value?.variables))

async function load() {
  isLoading.value = true
  try {
    source.value = await snippet.getBuiltin(name.value, pluginId.value)
    content.value = source.value.content ?? ''
  }
  catch {
    router.replace({ path: '/sites/snippets', query: { tab: 'builtin' } })
  }
  finally {
    isLoading.value = false
  }
}

watch([name, pluginId], load, { immediate: true })

function copyAsSnippet() {
  router.push({ path: '/sites/snippets/add', query: { from: name.value, ...(pluginId.value ? { plugin_id: pluginId.value } : {}) } })
}
</script>

<template>
  <BaseEditor :loading="isLoading">
    <template #left>
      <ACard variant="borderless">
        <template #title>
          <div class="flex min-w-0 flex-wrap items-center gap-x-2 gap-y-1">
            <span class="truncate">{{ source?.name || name }}</span>
            <ATag
              v-if="pluginId"
              color="blue"
              :bordered="false"
              class="m-0 font-normal"
            >
              {{ $gettext('From plugin %{id}', { id: pluginId }) }}
            </ATag>
            <ATag
              v-else
              :bordered="false"
              class="m-0 font-normal"
            >
              {{ $gettext('Built-in') }}
            </ATag>
            <ATag
              :bordered="false"
              class="m-0 font-normal"
            >
              {{ $gettext('Read Only') }}
            </ATag>
          </div>
        </template>
        <SnippetCode
          v-model="content"
          :rows="rows"
          :lines="[8, 40]"
          readonly
        />
        <section class="mt-6">
          <div class="mb-3 flex flex-wrap items-baseline gap-x-2">
            <span class="text-base font-medium">{{ $gettext('Preview') }}</span>
            <span class="hint text-sm">{{ $gettext('Updates as the snippet changes.') }}</span>
          </div>
          <SnippetPreview
            :rows="rows"
            :content="content"
            :from-plugin="!!pluginId"
          />
        </section>
      </ACard>
    </template>

    <template #right>
      <ACard
        variant="borderless"
        :title="$gettext('Details')"
      >
        <div class="flex flex-col gap-4">
          <AAlert
            v-if="pluginId"
            type="info"
            show-icon
            :title="$gettext('This template comes from the plugin %{id} and cannot be changed. Copy it as a snippet to change it, and review its directives before using it.', { id: pluginId })"
          />
          <AAlert
            v-else
            type="info"
            show-icon
            :title="$gettext('Built-in templates come with Nginx UI and cannot be changed. Copy one as a snippet to change it.')"
          />
          <div
            v-if="describe(source?.description)"
            class="flex flex-col gap-1"
          >
            <span class="hint text-[13px]">{{ $gettext('Description') }}</span>
            <span>{{ describe(source?.description) }}</span>
          </div>
          <div
            v-if="source?.author"
            class="flex flex-col gap-1"
          >
            <span class="hint text-[13px]">{{ $gettext('Author') }}</span>
            <span>{{ source.author }}</span>
          </div>
          <div class="flex flex-col gap-1">
            <span class="hint text-[13px]">{{ $gettext('File Name') }}</span>
            <span class="font-mono text-sm">{{ name }}</span>
          </div>
          <div
            v-if="rows.length > 0"
            class="flex flex-col gap-2"
          >
            <span class="hint text-[13px]">{{ $gettext('Variables') }}</span>
            <VariableSummary
              :rows="rows"
              detailed
            />
          </div>
        </div>
      </ACard>
    </template>
  </BaseEditor>

  <FooterToolBar>
    <ASpace>
      <AButton @click="router.push({ path: '/sites/snippets', query: { tab: 'builtin' } })">
        {{ $gettext('Back') }}
      </AButton>
      <AButton
        type="primary"
        :disabled="!source"
        @click="copyAsSnippet"
      >
        <template #icon>
          <CopyOutlined />
        </template>
        {{ $gettext('Copy as Snippet') }}
      </AButton>
    </ASpace>
  </FooterToolBar>
</template>

<style scoped lang="less">
.hint {
  color: var(--ant-color-text-secondary);
}
</style>
