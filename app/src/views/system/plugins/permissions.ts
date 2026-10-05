import type { Translate } from '@nginxui/plugin-market-ui'
import {
  describePermission as describeWith,
  permissionLabel as labelWith,
} from '@nginxui/plugin-market-ui'

export { permissionReasons } from '@nginxui/plugin-market-ui'
export type { PermissionReasonSource } from '@nginxui/plugin-market-ui'

const t: Translate = (msgid, params) => $gettext(msgid, params)

/**
 * Plain English explanation of a host permission, so the install and
 * approval dialogs say what granting it actually allows. The wording is the
 * marketplace's, shared with the developer portal.
 */
export function describePermission(permission: string): string {
  return describeWith(t, permission)
}

/** Short name of a permission, never the raw permission id. */
export function permissionLabel(permission: string): string {
  return labelWith(t, permission)
}
