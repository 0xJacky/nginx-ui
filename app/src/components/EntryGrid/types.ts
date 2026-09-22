import type { Component } from 'vue'
import type { RouteLocationRaw } from 'vue-router'

export interface EntryItem {
  /** Already translated card heading. */
  title: string
  /** Short sentence explaining what the destination is for. */
  description?: string
  /** Where the card navigates to. */
  path: RouteLocationRaw
  /**
   * Optional leading icon. Entries without one still align correctly, so a
   * grid may mix decorated and plain cards.
   */
  icon?: Component
  /** List key. Required when `path` is not a plain string. */
  key?: string
}
