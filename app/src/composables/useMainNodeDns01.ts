import type { MaybeRefOrGetter } from 'vue'
import auto_cert, { AutoCertChallengeMethod } from '@/api/auto_cert'
import { useSettingsStore } from '@/pinia'

/** Where a DNS-01 challenge of the selected node is validated. */
export type DnsVerifyOn = 'main' | 'node'

/**
 * With a node selected, DNS-01 can run on the main node, which then sends the
 * certificate to the node and keeps renewing it, so the node needs no plugin.
 * `enabled` turns the choice off where it does not apply, e.g. a renewal.
 */
export function useMainNodeDns01(enabled: MaybeRefOrGetter<boolean> = true) {
  const settings = useSettingsStore()

  const canVerifyOnMain = computed(() => settings.is_remote && toValue(enabled))

  // Whether the main node can run DNS-01, asked once a node is selected.
  const mainDns01 = ref<'unknown' | 'available' | 'missing'>('unknown')

  async function load() {
    try {
      const methods = await auto_cert.get_challenge_methods({ skipNodeProxy: true })
      mainDns01.value = methods.some(m => m.code === AutoCertChallengeMethod.dns01) ? 'available' : 'missing'
    }
    catch {
      mainDns01.value = 'unknown'
    }
  }

  watch(canVerifyOnMain, value => {
    if (value)
      load()
  }, { immediate: true })

  const verifyOptions = computed(() => [
    { label: $gettext('On the main node'), value: 'main' },
    { label: $gettext('On this node'), value: 'node' },
  ])

  function verifyHint(verifyOn: DnsVerifyOn) {
    return verifyOn === 'main'
      ? $gettext('The main node validates the domain with its DNS credentials, sends the certificate to this node and keeps renewing it.')
      : $gettext('This node validates the domain with its own DNS credentials and renews the certificate itself.')
  }

  return {
    canVerifyOnMain,
    mainDns01,
    nodeId: computed(() => settings.node.id),
    verifyOptions,
    verifyHint,
  }
}
