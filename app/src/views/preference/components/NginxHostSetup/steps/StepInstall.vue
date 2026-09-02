<script setup lang="ts">
import type { RenderedSnippets } from '@/api/host_setup'
import { ReloadOutlined } from '@antdv-next/icons'
import { computed, onActivated, onDeactivated, ref, watch } from 'vue'
import hostSetup from '@/api/host_setup'
import { getErrorMessage } from '@/lib/http'
import CheckPanel from '../CheckPanel.vue'
import CodeBlock from '../CodeBlock.vue'
import { useHostSetupWizard } from '../useHostSetupWizard'
import { useLatestRequest } from '../useLatestRequest'

const { isHostSetupPassed, params, requestParams } = useHostSetupWizard()

const snippets = ref<RenderedSnippets | null>(null)
const activeSide = ref<'host' | 'container'>('host')
const containerFormat = ref<'compose' | 'override' | 'docker-run'>('compose')
const { error: loadError, invalidate, isLoading, run } = useLatestRequest()

const isLaunchd = computed(() => params.value.service_manager === 'launchd')
const isMounted = computed(() => params.value.access_mode === 'mounted')
const needsHostGatewayMapping = computed(() =>
  !isLaunchd.value && params.value.host_address.startsWith('host.docker.internal'),
)
const needsContainerChange = computed(() => isMounted.value || needsHostGatewayMapping.value)
const sudoersFile = '/etc/sudoers.d/nginx-ui'
const sudoersInstallCommand = computed(() => `sudo visudo -f ${sudoersFile}`)
const authorizedKeysFile = computed(() => `~${params.value.host_user}/.ssh/authorized_keys`)

// The backend decides this: a launchd service and a root SSH user need none.
const needsSudoers = computed(() => snippets.value?.sudoers_required ?? false)
const hasHostCommands = computed(() => Boolean(snippets.value?.acl_commands?.trim()))
const aclOrder = computed(() => (needsSudoers.value ? 5 : 3))
// The numbering continues across both tabs so the whole install reads as one list.
const hostBlockCount = computed(() =>
  2 + (needsSudoers.value ? 2 : 0) + (hasHostCommands.value ? 1 : 0))
const containerSnippetOrder = computed(() => hostBlockCount.value + 1)
const applyOrder = computed(() => containerSnippetOrder.value + 1)

// docker run has no compose project to recreate, so the apply step differs.
const isDockerRun = computed(() => containerFormat.value === 'docker-run')
const applyCommand = computed(() => (isDockerRun.value
  ? 'docker rm -f nginx-ui'
  : 'docker compose up -d --force-recreate nginx-ui'))
const isHostSetupMinimal = computed(() => Boolean(snippets.value) && !needsSudoers.value && !hasHostCommands.value)

watch(needsContainerChange, needed => {
  if (!needed)
    activeSide.value = 'host'
})

async function refresh() {
  await run(() => hostSetup.preview(requestParams.value), {
    onSuccess: rendered => {
      snippets.value = rendered
    },
    onError: error => {
      snippets.value = null
      loadError.value = getErrorMessage(error)
    },
  })
}

watch(() => params.value.access_mode, () => void refresh())

onActivated(() => {
  void refresh()
})

onDeactivated(invalidate)
</script>

<template>
  <ASpin :spinning="isLoading">
    <AFormItem :label="$gettext('File access mode')">
      <ARadioGroup v-model:value="params.access_mode" option-type="button" button-style="solid">
        <ARadioButton value="sftp">
          {{ $gettext('Compatibility (SFTP)') }}
        </ARadioButton>
        <ARadioButton value="mounted">
          {{ $gettext('High performance (mounted)') }}
        </ARadioButton>
      </ARadioGroup>
      <div class="text-secondary mt-2 text-sm">
        {{ isMounted
          ? $gettext('Nginx UI reads configuration and logs from bind-mounted host directories. This is faster, but requires recreating the container.')
          : $gettext('Nginx UI reads configuration and logs entirely over SSH and SFTP. No host directories are mounted into the container.') }}
      </div>
    </AFormItem>

    <AAlert
      v-if="loadError"
      type="error"
      show-icon
      :title="$gettext('Failed to load setup instructions')"
      class="mb-4"
    >
      <template #description>
        <p class="mb-2">
          {{ loadError }}
        </p>
        <AButton size="small" @click="refresh">
          <ReloadOutlined />
          {{ $gettext('Retry') }}
        </AButton>
      </template>
    </AAlert>

    <AAlert
      v-if="!needsContainerChange"
      type="success"
      show-icon
      :title="$gettext('No container changes are required in compatibility mode')"
      :description="$gettext('Complete the host permissions below, then run the setup checks.')"
      class="mb-3"
    />

    <ATabs
      v-model:active-key="activeSide"
      :items="[
        { key: 'host', label: $gettext('1. On the nginx host') },
        ...(needsContainerChange
          ? [{ key: 'container', label: $gettext('2. On the Nginx UI container') }]
          : []),
      ]"
    >
      <template #contentRender="{ item }">
        <ASpace v-if="item.key === 'host'" orientation="vertical" size="middle" class="w-full">
          <AAlert
            type="info"
            show-icon
            :title="needsSudoers
              ? $gettext('Run these on the machine that runs nginx, as a user who can use sudo.')
              : $gettext('Run these on the machine that runs nginx, as the SSH user configured above.')"
          />

          <CodeBlock
            v-if="snippets"
            :order="1"
            :code="snippets.authorized_keys_install"
            language="shell"
            :title="$gettext('Install the public key')"
            :description="$gettext('Creates %{file} with the permissions sshd requires and appends the key. sshd silently ignores an authorized_keys file that other users can write, which shows up later as a permission denied error.', { file: authorizedKeysFile })"
          />

          <CodeBlock
            v-if="snippets"
            :order="2"
            :code="snippets.authorized_keys"
            language="ssh"
            :title="$gettext('Public key')"
            :description="$gettext('The same key on its own, for appending by hand. To restrict where it may be used from, prefix it with from=\'…\'.')"
          />

          <template v-if="needsSudoers && snippets">
            <CodeBlock
              :order="3"
              :code="sudoersInstallCommand"
              language="shell"
              :title="$gettext('Open the sudoers file')"
              :description="$gettext('visudo checks the syntax before saving, so a typo cannot lock you out of sudo.')"
            />
            <CodeBlock
              :order="4"
              :code="snippets.sudoers"
              language="sudoers"
              :title="$gettext('sudoers rules')"
              :description="$gettext('Paste this into the editor visudo opened, then save and exit. It lets the SSH user reload nginx without a password.')"
            />
          </template>

          <CodeBlock
            v-if="snippets && hasHostCommands"
            :order="aclOrder"
            :code="snippets.acl_commands"
            language="shell"
            :title="isLaunchd ? $gettext('File access check') : $gettext('File permissions')"
            :description="isLaunchd
              ? $gettext('Run as the SSH user. Both commands only test access and change nothing. A failure means the SSH user cannot reach the Homebrew paths.')
              : $gettext('Run as root. Grants the SSH user write access to the nginx config directory and read access to the logs, including files created later, plus read access to the PID file.')"
          />

          <AAlert
            v-if="isHostSetupMinimal"
            type="success"
            show-icon
            :title="$gettext('Nothing else to install on the host')"
            :description="isLaunchd
              ? $gettext('A Homebrew launchd service runs as the SSH user, so no sudoers entry or extra permissions are needed.')
              : $gettext('The SSH user is root, so no sudoers entry or extra permissions are needed.')"
          />
        </ASpace>

        <ASpace v-else-if="item.key === 'container'" orientation="vertical" size="middle" class="w-full">
          <AAlert
            type="info"
            show-icon
            :title="isMounted
              ? $gettext('Pick the format that matches how you run Nginx UI, then recreate the container with the bind mounts.')
              : $gettext('Linux Docker Engine requires this host-gateway mapping for host.docker.internal. No directory mounts are added.')"
          />

          <AAlert
            type="warning"
            show-icon
            :title="$gettext('Recreating the container closes this page')"
          >
            <template #description>
              <ul class="m-0 pl-4">
                <li>{{ $gettext('Finish the host side first, then come back here.') }}</li>
                <li>{{ $gettext('Restarting Nginx UI ends the two-factor session and clears anything typed in the earlier steps.') }}</li>
                <li>{{ $gettext('The generated key and known_hosts survive, because they live under the persisted /etc/nginx-ui directory.') }}</li>
                <li>{{ $gettext('Note the host address and SSH user before you recreate, then reopen the wizard and verify again.') }}</li>
              </ul>
            </template>
          </AAlert>

          <ATabs
            v-model:active-key="containerFormat"
            size="small"
            :items="[
              { key: 'compose', label: $gettext('docker compose') },
              { key: 'override', label: $gettext('override file') },
              { key: 'docker-run', label: $gettext('docker run') },
            ]"
          >
            <template #contentRender="{ item }">
              <CodeBlock
                v-if="item.key === 'compose' && snippets"
                :order="containerSnippetOrder"
                :code="snippets.compose_snippet"
                language="yaml"
                :title="$gettext('docker-compose.yml')"
                :description="isMounted
                  ? $gettext('Merge this under services.nginx-ui in your existing file. Each bind mount uses the same path inside the container as on the host.')
                  : $gettext('Merge this host-gateway mapping under services.nginx-ui in your existing file.')"
              />
              <CodeBlock
                v-else-if="item.key === 'override' && snippets"
                :order="containerSnippetOrder"
                :code="snippets.compose_override"
                language="yaml"
                :title="$gettext('docker-compose.override.yml')"
                :description="$gettext('Save as a new file next to your main compose file. Docker Compose merges it automatically, so your original file stays untouched.')"
              />
              <CodeBlock
                v-else-if="item.key === 'docker-run' && snippets"
                :order="containerSnippetOrder"
                :code="snippets.docker_run"
                language="shell"
                :title="$gettext('docker run')"
                :description="$gettext('Replaces your current docker run command. Adjust the published port and the image tag to match your current deployment.')"
              />
            </template>
          </ATabs>

          <CodeBlock
            v-if="snippets"
            :order="applyOrder"
            :code="applyCommand"
            language="shell"
            :title="$gettext('Apply the change')"
            :description="isMounted
              ? (isDockerRun
                ? $gettext('Remove the old container, then run the command above with the mounted host directories.')
                : $gettext('The bind mounts take effect once the container is recreated.'))
              : $gettext('The host-gateway mapping takes effect once the container is recreated.')"
          />
        </ASpace>
      </template>
    </ATabs>

    <CheckPanel
      v-model:passed="isHostSetupPassed"
      class="mt-4"
      :groups="['platform', 'privileges']"
      :title="$gettext('Setup checks')"
      :hint="$gettext('Run these after applying the relevant host and container instructions above. They verify the service, file access and required privileges for the selected mode.')"
      :disabled="isLoading || Boolean(loadError)"
    />
  </ASpin>
</template>
