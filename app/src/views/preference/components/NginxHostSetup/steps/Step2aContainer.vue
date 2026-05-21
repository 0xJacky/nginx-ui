<script setup lang="ts">
import type { RenderedSnippets, SetupParams } from '@/api/host_setup'
import { onMounted, ref, watch } from 'vue'
import hostSetup from '@/api/host_setup'
import CodeBlock from '../CodeBlock.vue'

const props = defineProps<{ params: SetupParams }>()
const snippets = ref<RenderedSnippets | null>(null)
const activeTab = ref<'compose' | 'override' | 'docker-run'>('compose')

async function refresh() {
  snippets.value = await hostSetup.preview(props.params)
}

onMounted(refresh)
watch(() => props.params, refresh, { deep: true })
</script>

<template>
  <div>
    <AAlert
      type="info"
      show-icon
      :message="$gettext('Container side — choose one of the three formats below')"
      class="mb-4"
    />
    <ATabs v-model:active-key="activeTab">
      <ATabPane key="compose" :tab="$gettext('docker-compose snippet')">
        <CodeBlock
          v-if="snippets"
          :code="snippets.compose_snippet"
          language="yaml"
          :title="$gettext('Merge into services.nginx-ui')"
        />
      </ATabPane>
      <ATabPane key="override" :tab="$gettext('override file')">
        <CodeBlock
          v-if="snippets"
          :code="snippets.compose_override"
          language="yaml"
          :title="$gettext('Save as docker-compose.override.yml')"
        />
      </ATabPane>
      <ATabPane key="docker-run" :tab="$gettext('docker run')">
        <CodeBlock
          v-if="snippets"
          :code="snippets.docker_run"
          language="shell"
        />
      </ATabPane>
    </ATabs>
  </div>
</template>
