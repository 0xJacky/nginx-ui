<script setup lang="ts">
import type { RenderedSnippets, SetupParams } from '@/api/host_setup'
import { onMounted, ref, watch } from 'vue'
import hostSetup from '@/api/host_setup'
import CodeBlock from '../CodeBlock.vue'

const props = defineProps<{ params: SetupParams }>()
const snippets = ref<RenderedSnippets | null>(null)

async function refresh() {
  snippets.value = await hostSetup.preview(props.params)
}

onMounted(refresh)
watch(() => props.params, refresh, { deep: true })
</script>

<template>
  <div class="space-y-4">
    <AAlert
      type="info"
      show-icon
      :message="$gettext('Host side — run these on the machine that runs nginx')"
    />

    <CodeBlock
      v-if="snippets"
      :code="snippets.sudoers"
      language="sudoers"
      :title="$gettext('/etc/sudoers.d/nginx-ui (sudo visudo -f)')"
    />
    <CodeBlock
      v-if="snippets"
      :code="snippets.authorized_keys"
      language="ssh"
      :title="$gettext('Append to ~nginxui/.ssh/authorized_keys')"
    />
    <CodeBlock
      v-if="snippets"
      :code="snippets.acl_commands"
      language="shell"
      :title="$gettext('ACL commands (run as root)')"
    />
  </div>
</template>
