import type { InjectionKey, Ref } from 'vue'
import type { SetupParams } from '@/api/host_setup'
import type { Settings } from '@/api/settings'
import { computed, inject, provide, ref, watch } from 'vue'

export const hostSetupStepOrder = [
  'ssh-target',
  'trust-and-test',
  'detect-platform',
  'install',
  'verify',
] as const

export type HostSetupStepId = typeof hostSetupStepOrder[number]

/** Params that automatic detection can fill in. */
export const detectableFields = [
  'service_manager',
  'systemctl_path',
  'launchctl_path',
  'systemd_unit',
  'launchd_service',
  'nginx_sbin_path',
  'host_config_dir',
  'host_log_dir',
  'pid_path',
] as const

export type DetectableField = typeof detectableFields[number]

/** Detectable fields rendered as a free-text path input. */
export type DetectablePathField = Exclude<DetectableField, 'service_manager'>

/** Where the current value of a detectable field came from. */
export type FieldOrigin = 'detected' | 'overridden' | 'unknown'

export type KeySource = 'generated' | 'existing' | 'provided'

const defaultHostPrivateKeyPath = '/etc/nginx-ui/host_key'
const defaultHostKnownHostsPath = '/etc/nginx-ui/known_hosts'

function hasValue(value?: string) {
  return Boolean(value?.trim())
}

// The backend serialises an unset key source as "". Nullish coalescing would
// keep that empty string and leave no source selected.
function normalizeKeySource(source: string | undefined, privateKeyPath: string): KeySource {
  if (source === 'generated' || source === 'existing' || source === 'provided')
    return source
  return privateKeyPath === defaultHostPrivateKeyPath ? 'generated' : 'existing'
}

function initialAccessMode(nginx: Settings['nginx']): SetupParams['access_mode'] {
  if (nginx.host_mode !== 'ssh')
    return 'sftp'
  if (nginx.host_access_mode === 'sftp' || nginx.host_access_mode === 'mounted')
    return nginx.host_access_mode
  throw new Error('Saved SSH settings do not contain a valid host access mode')
}

export function createHostSetupWizard(settings: Ref<Settings>) {
  const nginx = settings.value.nginx
  const serviceManager = nginx.host_service_manager || 'systemd'
  const configuredPrivateKeyPath = nginx.host_private_key_path || defaultHostPrivateKeyPath
  const configuredKeySource = normalizeKeySource(nginx.host_key_source, configuredPrivateKeyPath)
  const currentStepId = ref<HostSetupStepId>('ssh-target')
  const isHostIdentityTrusted = ref(false)
  const isSSHConnected = ref(false)
  const isHostSetupPassed = ref(false)
  const isVerificationPassed = ref(false)
  // SSH key is the only method the backend accepts, so there is nothing to ask.
  const authMethod = 'key' as const
  const publicKey = ref('')
  const privateKeyOnce = ref('')
  const validatedPrivateKeyPath = ref('')
  const params = ref<SetupParams>({
    host_address: nginx.host_address || 'host.docker.internal:22',
    host_user: nginx.host_user || 'nginxui',
    access_mode: initialAccessMode(nginx),
    service_manager: serviceManager,
    systemd_unit: nginx.host_systemd_unit_name || 'nginx.service',
    systemctl_path: nginx.host_systemctl_path || '/bin/systemctl',
    launchd_service: nginx.host_launchd_service || 'homebrew.mxcl.nginx',
    launchctl_path: nginx.host_launchctl_path || '/bin/launchctl',
    nginx_sbin_path: nginx.sbin_path || (serviceManager === 'launchd' ? '/opt/homebrew/opt/nginx/bin/nginx' : '/usr/sbin/nginx'),
    host_config_dir: nginx.host_config_dir || (serviceManager === 'launchd' ? '/opt/homebrew/etc/nginx' : '/etc/nginx'),
    host_log_dir: nginx.host_log_dir || (serviceManager === 'launchd' ? '/opt/homebrew/var/log/nginx' : '/var/log/nginx'),
    pid_path: nginx.pid_path || (serviceManager === 'launchd' ? '/opt/homebrew/var/run/nginx.pid' : '/var/run/nginx.pid'),
    container_key_path: configuredPrivateKeyPath,
    container_known_hosts_path: nginx.host_known_hosts_path || defaultHostKnownHostsPath,
    key_source: configuredKeySource,
    use_generated_key: configuredKeySource !== 'existing',
    public_key_open_ssh: '',
  })

  // Values reported by the target host, used to mark each field as detected or
  // overridden and to restore the detected value.
  const detectedValues = ref<Partial<Record<DetectableField, string>>>({})

  function recordDetected(patch: Partial<SetupParams>) {
    for (const field of detectableFields) {
      const value = patch[field]
      if (typeof value === 'string' && value)
        detectedValues.value[field] = value
    }
  }

  function detectedValue(field: DetectableField) {
    return detectedValues.value[field] ?? ''
  }

  function fieldOrigin(field: DetectableField): FieldOrigin {
    const detected = detectedValues.value[field]
    if (!detected)
      return 'unknown'
    return params.value[field] === detected ? 'detected' : 'overridden'
  }

  function restoreDetected(field: DetectableField) {
    const detected = detectedValues.value[field]
    if (detected)
      params.value = { ...params.value, [field]: detected }
  }

  watch(publicKey, value => {
    params.value.public_key_open_ssh = value
  })

  watch(
    [() => params.value.container_key_path, () => params.value.use_generated_key],
    () => {
      const currentPath = params.value.container_key_path?.trim() ?? ''
      if (validatedPrivateKeyPath.value !== currentPath) {
        validatedPrivateKeyPath.value = ''
        publicKey.value = ''
      }
      privateKeyOnce.value = ''
    },
  )

  watch(
    () => params.value.host_address,
    () => {
      isHostIdentityTrusted.value = false
      isSSHConnected.value = false
      isVerificationPassed.value = false
    },
  )

  watch(
    [() => params.value.host_user, publicKey],
    () => {
      isSSHConnected.value = false
      isVerificationPassed.value = false
    },
  )

  watch(params, () => {
    isHostSetupPassed.value = false
    isVerificationPassed.value = false
  }, { deep: true })

  const currentStepIndex = computed(() => hostSetupStepOrder.indexOf(currentStepId.value))
  const isAuthenticationReady = computed(() => {
    const privateKeyPath = params.value.container_key_path?.trim() ?? ''
    return hasValue(publicKey.value)
      && hasValue(privateKeyPath)
      && validatedPrivateKeyPath.value === privateKeyPath
  })
  const isTargetReady = computed(() => {
    return hasValue(params.value.host_address) && hasValue(params.value.host_user) && isAuthenticationReady.value
  })

  // The platform step accepts any complete set of paths. Detection is advisory.
  const isPlatformReady = computed(() => {
    const p = params.value
    if (!hasValue(p.service_manager) || !hasValue(p.nginx_sbin_path))
      return false
    if (!hasValue(p.host_config_dir) || !hasValue(p.host_log_dir) || !hasValue(p.pid_path))
      return false
    return p.service_manager === 'launchd'
      ? hasValue(p.launchd_service) && hasValue(p.launchctl_path)
      : hasValue(p.systemd_unit) && hasValue(p.systemctl_path)
  })

  // Human-readable reason the current step cannot be left yet, or '' when it can.
  const blockedReason = computed(() => {
    switch (currentStepId.value) {
      case 'ssh-target':
        if (!hasValue(params.value.host_address))
          return $gettext('Enter the SSH host address.')
        if (!hasValue(params.value.host_user))
          return $gettext('Enter the SSH user.')
        if (!isAuthenticationReady.value)
          return $gettext('Generate, validate or import a private key first.')
        return ''
      case 'trust-and-test':
        if (!isHostIdentityTrusted.value)
          return $gettext('Trust every presented host key first.')
        if (!isSSHConnected.value)
          return $gettext('Run the SSH connection test first.')
        return ''
      case 'detect-platform':
        if (!isPlatformReady.value)
          return $gettext('Fill in the platform and every nginx path below.')
        return ''
      default:
        return ''
    }
  })

  const canAdvance = computed(() => currentStepId.value === 'install'
    ? isHostSetupPassed.value
    : blockedReason.value === '')

  // Steps the user already completed stay reachable so nothing is a dead end.
  const furthestReachableIndex = computed(() => {
    if (!isTargetReady.value)
      return 0
    if (!isHostIdentityTrusted.value || !isSSHConnected.value)
      return 1
    if (!isPlatformReady.value)
      return 2
    if (!isHostSetupPassed.value)
      return 3
    return hostSetupStepOrder.length - 1
  })

  function goToStep(index: number) {
    const step = hostSetupStepOrder[index]
    if (!step)
      return
    if (index > currentStepIndex.value && index > furthestReachableIndex.value)
      return
    currentStepId.value = step
  }

  function next() {
    if (!canAdvance.value)
      return

    const nextStep = hostSetupStepOrder[currentStepIndex.value + 1]
    if (nextStep)
      currentStepId.value = nextStep
  }

  function previous() {
    const previousStep = hostSetupStepOrder[currentStepIndex.value - 1]
    if (previousStep)
      currentStepId.value = previousStep
  }

  function clearSensitiveState() {
    privateKeyOnce.value = ''
  }

  function applyToSettings() {
    if (!isVerificationPassed.value)
      return false

    const target = settings.value.nginx
    target.host_mode = 'ssh'
    target.host_access_mode = params.value.access_mode
    target.host_address = params.value.host_address
    target.host_user = params.value.host_user
    target.host_auth_method = authMethod
    target.host_key_source = params.value.key_source || 'generated'
    target.host_private_key_path = params.value.container_key_path?.trim() || defaultHostPrivateKeyPath
    target.host_known_hosts_path ||= defaultHostKnownHostsPath
    target.host_service_manager = params.value.service_manager
    target.host_sudo_prefix = params.value.service_manager === 'launchd'
      ? ''
      : (target.host_sudo_prefix || 'sudo -n')
    target.host_systemd_unit_name = params.value.systemd_unit
    target.host_systemctl_path = params.value.systemctl_path
    target.host_launchd_service = params.value.launchd_service
    target.host_launchctl_path = params.value.launchctl_path
    target.host_config_dir = params.value.host_config_dir
    target.host_log_dir = params.value.host_log_dir
    target.sbin_path = params.value.nginx_sbin_path ?? target.sbin_path
    target.pid_path = params.value.pid_path ?? target.pid_path
    target.config_dir = params.value.host_config_dir ?? target.config_dir
    target.config_path = `${params.value.host_config_dir ?? '/etc/nginx'}/nginx.conf`
    target.error_log_path = `${params.value.host_log_dir ?? '/var/log/nginx'}/error.log`
    target.access_log_path = `${params.value.host_log_dir ?? '/var/log/nginx'}/access.log`
    clearSensitiveState()
    return true
  }

  return {
    applyToSettings,
    authMethod,
    blockedReason,
    canAdvance,
    clearSensitiveState,
    currentStepId,
    currentStepIndex,
    detectedValue,
    fieldOrigin,
    furthestReachableIndex,
    goToStep,
    isHostIdentityTrusted,
    isHostSetupPassed,
    isPlatformReady,
    isSSHConnected,
    isVerificationPassed,
    next,
    params,
    previous,
    privateKeyOnce,
    publicKey,
    recordDetected,
    restoreDetected,
    validatedPrivateKeyPath,
  }
}

export type HostSetupWizard = ReturnType<typeof createHostSetupWizard>

const hostSetupWizardKey: InjectionKey<HostSetupWizard> = Symbol('host-setup-wizard')

export function provideHostSetupWizard(wizard: HostSetupWizard) {
  provide(hostSetupWizardKey, wizard)
}

export function useHostSetupWizard() {
  const wizard = inject(hostSetupWizardKey)
  if (!wizard)
    throw new Error('Host setup wizard context is not available')
  return wizard
}
