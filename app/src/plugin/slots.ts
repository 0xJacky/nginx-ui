import type { SlotContext, SlotRegistration } from './types'

/** Slot name prefixes the log pages read as families of registrations. */
export const NGINX_LOG_VIEW_SLOT_PREFIX = 'nginx_log.view:'
export const NGINX_LOG_COLUMN_SLOT_PREFIX = 'nginx_log.list.column:'

/** One registration of a slot family, with the key taken from the slot name. */
export interface PrefixedSlotRegistration {
  /** Full slot name, for example `nginx_log.view:search`. */
  slot: string
  /** The part of the slot name after the prefix. */
  key: string
  registration: SlotRegistration
}

/**
 * Whether a registration applies to `ctx`. A `when` that throws counts as
 * false, so a broken plugin cannot break the page that renders its slot.
 */
export function registrationApplies(registration: SlotRegistration, ctx: SlotContext = {}): boolean {
  if (!registration.when)
    return true

  try {
    return registration.when(ctx) !== false
  }
  catch (error) {
    console.error(`[plugin] ${registration.pluginId}: slot condition failed`, error)
    return false
  }
}

/**
 * Every registration whose slot name starts with `prefix` and has a non empty
 * key, ordered by `order` and then by registration order.
 */
export function collectSlotsByPrefix(
  slots: Record<string, SlotRegistration[]>,
  prefix: string,
  ctx: SlotContext = {},
): PrefixedSlotRegistration[] {
  const found: PrefixedSlotRegistration[] = []

  for (const [slot, registrations] of Object.entries(slots)) {
    if (!slot.startsWith(prefix))
      continue

    const key = slot.slice(prefix.length)
    if (!key)
      continue

    for (const registration of registrations) {
      if (registrationApplies(registration, ctx))
        found.push({ slot, key, registration })
    }
  }

  // Array.prototype.sort is stable, so equal orders keep registration order.
  return found.sort((a, b) => a.registration.order - b.registration.order)
}

/** Keeps the first registration of each key, which is how a mode switch lists them. */
export function uniqueByKey(items: PrefixedSlotRegistration[]): PrefixedSlotRegistration[] {
  const seen = new Set<string>()
  return items.filter(item => {
    if (seen.has(item.key))
      return false
    seen.add(item.key)
    return true
  })
}
