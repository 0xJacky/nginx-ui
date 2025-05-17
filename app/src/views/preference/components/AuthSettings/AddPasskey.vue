<script setup lang="ts">
import { startRegistration } from '@simplewebauthn/browser'
import { message } from 'ant-design-vue'
import passkey from '@/api/passkey'
import { useUserStore } from '@/pinia'

const emit = defineEmits(['created'])

const user = useUserStore()
const passkeyName = ref('')
const addPasskeyModelOpen = ref(false)
const passkeyEnabled = ref(false)

const regLoading = ref(false)
async function registerPasskey() {
  regLoading.value = true
  const optionsJSON = await passkey.begin_registration()

  const attestationResponse = await startRegistration({ optionsJSON })

  await passkey.finish_registration(attestationResponse, passkeyName.value)

  emit('created')

  message.success($gettext('Register passkey successfully'))
  addPasskeyModelOpen.value = false

  user.passkeyRawId = attestationResponse.rawId
  regLoading.value = false
}

function addPasskey() {
  addPasskeyModelOpen.value = true
  passkeyName.value = ''
}

passkey.get_config_status().then(r => {
  passkeyEnabled.value = r.status
})
</script>

<template>
  <div>
    <AButton @click="addPasskey">
      {{ $gettext('Add a passkey') }}
    </AButton>
    <AModal
      v-model:open="addPasskeyModelOpen"
      :title="$gettext('Add a passkey')"
      centered
      :mask="false"
      :mask-closable="!passkeyEnabled"
      :closable="!passkeyEnabled"
      :footer="passkeyEnabled ? undefined : false"
      :confirm-loading="regLoading"
      @ok="registerPasskey"
    >
      <AForm
        v-if="passkeyEnabled"
        layout="vertical"
      >
        <div>
          <AAlert
            class="mb-4"
            :message="$gettext('Tips')"
            type="info"
          >
            <template #description>
              <p>{{ $gettext('Please enter a name for the passkey you wish to create and click the OK button below.') }}</p>
              <p>{{ $gettext('If your browser supports WebAuthn Passkey, a dialog box will appear.') }}</p>
              <p>{{ $gettext('Follow the instructions in the dialog to complete the passkey registration process.') }}</p>
            </template>
          </AAlert>
        </div>
        <AFormItem :label="$gettext('Name')">
          <AInput v-model:value="passkeyName" />
        </AFormItem>
      </AForm>
      <div v-else>
        <AAlert
          class="mb-4"
          :message="$gettext('Warning')"
          type="warning"
          show-icon
        >
          <template #description>
            <p>{{ $gettext('You have not configured the settings of Webauthn, so you cannot add a passkey.') }}</p>
            <p>
              {{ $gettext('To ensure security, Webauthn configuration cannot be added through the UI. '
                + 'Please manually configure the following in the app.ini configuration file and restart Nginx UI.') }}
            </p>
            <pre>[webauthn]
# This is the display name
RPDisplayName = Nginx UI
# The domain name of Nginx UI
RPID          = localhost
# The list of origin addresses
RPOrigins     = http://localhost:3002</pre>
            <p>{{ $gettext('Afterwards, refresh this page and click add passkey again.') }}</p>
            <p>
              {{ $gettext(`Due to the security policies of some browsers, you cannot use passkeys on non-HTTPS websites, except when running on localhost.`) }}
            </p>
          </template>
        </AAlert>
      </div>
    </AModal>
  </div>
</template>

<style scoped lang="less">

</style>
