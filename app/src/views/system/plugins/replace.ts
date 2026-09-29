import type { MaybeRefOrGetter } from 'vue'
import type { TrustedOffer } from './marketplace/trust'
import type { PluginInfo } from '@/api/plugin'
import { replacePlugin } from '@/api/plugin_marketplace'
import { getErrorMessage } from '@/lib/http'
import { usePluginLoader } from '@/plugin'
import { usePluginInventory } from './inventory'
import { findTrustedOffer, trustedOfferAction } from './marketplace/trust'

/** The more trusted marketplace package of an installed plugin, if any. */
export function useTrustedOffer(plugin: MaybeRefOrGetter<PluginInfo | undefined>) {
  const inventory = usePluginInventory()
  return computed(() => {
    const current = toValue(plugin)
    if (!current)
      return undefined
    const entry = inventory.catalog.value.find(item => item.id === current.id)
    return findTrustedOffer(current.trust, entry)
  })
}

/**
 * Confirms and then replaces an installed plugin with the package the offer
 * points at. Settings and data stay.
 */
export function useReplacePlugin() {
  const { message, modal } = useGlobalApp()
  const inventory = usePluginInventory()
  const pluginLoader = usePluginLoader()
  const replacingId = ref('')

  async function replace(offer: TrustedOffer) {
    const id = offer.entry.id
    replacingId.value = id
    try {
      const info = await replacePlugin(id, offer.entry.source)
      if (info.status === 'needs_approval')
        message.warning($gettext('Installed. Turn it on to review what it can access.'))
      else
        message.success($gettext('Installed the version from the marketplace'))
      await Promise.all([inventory.reload(true), inventory.reloadCatalog()])
      await pluginLoader.loadNew()
    }
    catch (e) {
      message.error(getErrorMessage(e, $gettext('Failed to install the version from the marketplace')))
    }
    finally {
      replacingId.value = ''
    }
  }

  function confirmReplace(offer: TrustedOffer) {
    modal.confirm({
      title: trustedOfferAction(offer),
      content: $gettext('Replace the installed version with the one from the marketplace? Settings and data are kept.'),
      okText: $gettext('Replace'),
      cancelText: $gettext('Cancel'),
      onOk: () => replace(offer),
    })
  }

  return { replacingId, confirmReplace }
}
