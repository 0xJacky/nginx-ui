<script setup lang="ts">
import { ref } from 'vue'
import hostSetup from '@/api/host_setup'
import CodeBlock from '../CodeBlock.vue'

const authMethod = defineModel<'key' | 'password'>('authMethod', { default: 'key' })
const publicKey = defineModel<string>('publicKey', { default: '' })

const generating = ref(false)
const privateKeyOnce = ref('')

async function regenerate() {
  generating.value = true
  try {
    const res = await hostSetup.generateKeypair()
    publicKey.value = res.public_key
    privateKeyOnce.value = res.private_key ?? ''
  }
  finally {
    generating.value = false
  }
}

async function loadExisting() {
  try {
    const res = await hostSetup.getPublicKey()
    publicKey.value = res.public_key
  }
  catch {
    publicKey.value = ''
  }
}

loadExisting()
</script>

<template>
  <div class="space-y-4">
    <AFormItem :label="$gettext('Authentication method')">
      <ARadioGroup v-model:value="authMethod">
        <ARadio value="key">
          {{ $gettext('SSH key (recommended)') }}
        </ARadio>
        <ARadio value="password" disabled>
          {{ $gettext('Password (not yet supported)') }}
        </ARadio>
      </ARadioGroup>
    </AFormItem>

    <div v-if="authMethod === 'key'">
      <AFormItem :label="$gettext('Public key')">
        <CodeBlock
          v-if="publicKey"
          :code="publicKey"
          language="ssh"
          :title="$gettext('Public key (paste into authorized_keys)')"
        />
        <AEmpty
          v-else
          :description="$gettext('No key generated yet')"
        />
        <div class="mt-3 flex gap-2">
          <AButton
            :loading="generating"
            @click="regenerate"
          >
            {{ publicKey ? $gettext('Regenerate keypair') : $gettext('Generate keypair') }}
          </AButton>
        </div>
      </AFormItem>

      <AAlert
        v-if="privateKeyOnce"
        type="warning"
        show-icon
        class="mt-3"
      >
        <template #message>
          {{ $gettext('Private key generated (shown once)') }}
        </template>
        <template #description>
          <p>{{ $gettext('Save this private key somewhere safe. It will NOT be shown again.') }}</p>
          <CodeBlock
            :code="privateKeyOnce"
            language="ssh"
          />
        </template>
      </AAlert>
    </div>
  </div>
</template>
