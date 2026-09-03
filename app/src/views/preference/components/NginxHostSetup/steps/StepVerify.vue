<script setup lang="ts">
import CheckPanel from '../CheckPanel.vue'
import { useHostSetupWizard } from '../useHostSetupWizard'

const { isVerificationPassed } = useHostSetupWizard()
</script>

<template>
  <CheckPanel
    v-model:passed="isVerificationPassed"
    :groups="['nginx']"
    :check-keys="['nginx_test']"
    :title="$gettext('Verification')"
    :hint="$gettext('The earlier steps already verified SSH, platform, file access and privileges. This final check only runs nginx -t against the host configuration.')"
    :error-title="$gettext('Verification failed')"
    results-outside-card
  >
    <template #action="{ run, running }">
      <AButton type="primary" size="small" :loading="running" @click="run()">
        {{ $gettext('Run verification') }}
      </AButton>
    </template>

    <template #default="{ run, running, result, runError, hasFailed }">
      <!--
        nginx -t is the only check that validates the host's own configuration
        rather than the SSH setup, so a failure here is offered an explicit
        override.
      -->
      <AAlert
        v-if="hasFailed('nginx_test')"
        type="warning"
        show-icon
        :message="$gettext('The nginx configuration on the host failed validation')"
      >
        <template #description>
          <p class="mb-2">
            {{ $gettext('This is the configuration already present on the host, not something this wizard wrote. The SSH setup itself can still be saved, and the configuration can be fixed from Nginx UI afterwards.') }}
          </p>
          <AButton size="small" danger ghost :loading="running" @click="run({ skipNginxT: true })">
            {{ $gettext('Continue without validating the configuration') }}
          </AButton>
        </template>
      </AAlert>

      <AEmpty
        v-if="!result && !running && !runError"
        :description="$gettext('Run the final nginx configuration check to enable saving.')"
      />
    </template>
  </CheckPanel>
</template>
