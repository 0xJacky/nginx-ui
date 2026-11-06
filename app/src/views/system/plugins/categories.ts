import type { Translate } from '@nginxui/plugin-market-ui'
import { categoryLabel as labelWith, sortCategories } from '@nginxui/plugin-market-ui'

export { sortCategories }

const t: Translate = (msgid, params) => $gettext(msgid, params)

/** Plain name of a catalog category; one this host does not know shows its id. */
export function categoryLabel(id: string): string {
  return labelWith(t, id)
}
