import type { Translate } from '@nginxui/plugin-market-ui'
import type { CatalogCommercial } from '@/api/plugin_marketplace'
import { commercialTerms as termsWith } from '@nginxui/plugin-market-ui'

export { commercialPrice } from '@nginxui/plugin-market-ui'

const t: Translate = (msgid, params) => $gettext(msgid, params)

/** The license kind and the trial of a paid plugin, as short phrases. */
export function commercialTerms(commercial: CatalogCommercial): string[] {
  return termsWith(t, commercial)
}
