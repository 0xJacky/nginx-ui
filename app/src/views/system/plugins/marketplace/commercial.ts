import type { CatalogCommercial } from '@/api/plugin_marketplace'
import { localizedText } from '@/api/plugin'

/** The price text of a paid plugin in the given language, English as fallback. */
export function commercialPrice(commercial: CatalogCommercial, language: string): string {
  return localizedText(commercial.pricing, language)
}

/** The license kind and the trial of a paid plugin, as short phrases. */
export function commercialTerms(commercial: CatalogCommercial): string[] {
  const terms: string[] = []
  if (commercial.license === 'subscription')
    terms.push($gettext('Subscription'))
  else if (commercial.license === 'commercial')
    terms.push($gettext('Paid license'))
  if (commercial.trial_days)
    terms.push($gettext('Free trial of %{days} days', { days: String(commercial.trial_days) }))
  return terms
}
