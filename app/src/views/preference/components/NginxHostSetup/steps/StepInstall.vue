<script setup lang="ts">
import type { RenderedSnippets } from '@/api/host_setup'
import { ReloadOutlined } from '@ant-design/icons-vue'
import { computed, onActivated, ref } from 'vue'
import hostSetup from '@/api/host_setup'
import { getErrorMessage } from '@/lib/http'
import CheckPanel from '../CheckPanel.vue'
import CodeBlock from '../CodeBlock.vue'
import { useHostSetupWizard } from '../useHostSetupWizard'

const { hasVisitedInstall, params } = useHostSetupWizard()

const snippets = ref<RenderedSnippets | null>(null)
const activeSide = ref<'host' | 'container'>('host')
const containerFormat = ref<'compose' | 'override' | 'docker-run'>('compose')
const isLoading = ref(false)
const loadError = ref('')

const isLaunchd = computed(() => params.value.service_manager === 'launchd')
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

async function refresh() {
  isLoading.value = true
  loadError.value = ''
  try {
    snippets.value = await hostSetup.preview(params.value)
  }
  catch (error) {
    snippets.value = null
    loadError.value = getErrorMessage(error)
  }
  finally {
    isLoading.value = false
  }
}

onActivated(() => {
  hasVisitedInstall.value = true
  void refresh()
})
</script>

<template>
  <ASpin :spinning="isLoading">
    <AAlert
      v-if="loadError"
      type="error"
      show-icon
      :message="$gettext('Failed to load setup instructions')"
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

    <ATabs v-model:active-key="activeSide">
      <ATabPane key="host" :tab="$gettext('1. On the nginx host')">
        <ASpace direction="vertical" size="middle" class="w-full">
          <AAlert
            type="info"
            show-icon
            :message="needsSudoers
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
            :message="$gettext('Nothing else to install on the host')"
            :description="isLaunchd
              ? $gettext('A Homebrew launchd service runs as the SSH user, so no sudoers entry or extra permissions are needed.')
              : $gettext('The SSH user is root, so no sudoers entry or extra permissions are needed.')"
          />
        </ASpace>
      </ATabPane>

      <ATabPane key="container" :tab="$gettext('2. On the Nginx UI container')">
        <ASpace direction="vertical" size="middle" class="w-full">
          <AAlert
            type="info"
            show-icon
            :message="$gettext('Pick the format that matches how you run Nginx UI, then recreate the container.')"
          />

          <AAlert
            type="warning"
            show-icon
            :message="$gettext('Recreating the container closes this page')"
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

          <ATabs v-model:active-key="containerFormat" size="small">
            <ATabPane key="compose" :tab="$gettext('docker compose')">
              <CodeBlock
                v-if="snippets"
                :order="containerSnippetOrder"
                :code="snippets.compose_snippet"
                language="yaml"
                :title="$gettext('docker-compose.yml')"
                :description="$gettext('Merge this under services.nginx-ui in your existing file. Each bind mount uses the same path inside the container as on the host.')"
              />
            </ATabPane>
            <ATabPane key="override" :tab="$gettext('override file')">
              <CodeBlock
                v-if="snippets"
                :order="containerSnippetOrder"
                :code="snippets.compose_override"
                language="yaml"
                :title="$gettext('docker-compose.override.yml')"
                :description="$gettext('Save as a new file next to your main compose file. Docker Compose merges it automatically, so your original file stays untouched.')"
              />
            </ATabPane>
            <ATabPane key="docker-run" :tab="$gettext('docker run')">
              <CodeBlock
                v-if="snippets"
                :order="containerSnippetOrder"
                :code="snippets.docker_run"
                language="shell"
                :title="$gettext('docker run')"
                :description="$gettext('Replaces your current docker run command. Adjust the published port and the image tag to match your current deployment.')"
              />
            </ATabPane>
          </ATabs>

          <CodeBlock
            v-if="snippets"
            :order="applyOrder"
            :code="applyCommand"
            language="shell"
            :title="$gettext('Apply the change')"
            :description="isDockerRun
              ? $gettext('Remove the old container, then run the command above. The new environment variables only take effect on a fresh container.')
              : $gettext('The new environment variables only take effect once the container is recreated.')"
          />
        </ASpace>
      </ATabPane>
    </ATabs>

    <CheckPanel
      v-if="needsSudoers"
      class="mt-4"
      group="privileges"
      :title="$gettext('Permission checks')"
      :hint="$gettext('Run this once the sudoers rules are installed on the host.')"
    />
  </ASpin>
</template>
