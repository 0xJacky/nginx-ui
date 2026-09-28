import type { Router } from 'vue-router'
import type { PaneMessage, ShellMessage } from './protocol'
import { watch } from 'vue'
import loadTranslations from '@/api/translations'
import gettext from '@/gettext'
import { loadDayjsLocale } from '@/lib/helper/dayjsLocale'
import { useSettingsStore } from '@/pinia'
import { tabIdFromWindowName } from './paneStorage'
import {
  createEnvelope,
  createShellEnvelope,
  hasStoredToken,
  isPaneEnvelope,
  isShellEnvelope,
  parseSharedSettings,
  SETTINGS_STORAGE_KEY,
  USER_STORAGE_KEY,
} from './protocol'

/*
 * Workspace shell <-> pane bridge.
 *
 * Pane -> shell goes through postMessage (see protocol.ts): focus, node and
 * route changes. Shell -> pane postMessage carries only whether the pane has
 * focus, which exists nowhere in storage. They are reported whenever they happen, not only on load,
 * because a pane may switch nodes or navigate without reloading.
 *
 * Shell -> pane (theme, language) and the shared login session go through the
 * `storage` event instead: every pane already persists these to the same
 * localStorage entries, the browser delivers the event to every other
 * same-origin document, and panes that are created later simply read the
 * current value at boot. No pane registry or re-broadcast is needed, and a
 * change made in one pane reaches the shell and its siblings the same way.
 */

// ---------------------------------------------------------------- pane side

/** Set on the pane's <html> while another pane has focus; styled in HeaderLayout. */
export const INACTIVE_PANE_CLASS = 'workspace-pane-inactive'

function postToShell(tabId: number, message: PaneMessage) {
  if (window.parent === window)
    return

  window.parent.postMessage(createEnvelope(tabId, message), location.origin)
}

/** Applies theme and language that the shell or another pane changed. */
async function applySharedSettings(raw: string | null, router: Router) {
  const settings = useSettingsStore()
  const shared = parseSharedSettings(raw)

  // Plain mutations in one synchronous block: the persisted-state subscriber
  // then writes the shared entry once, on the next flush, with every field
  // final, which equals what the other windows hold and fires no new storage
  // event. `$patch` would notify it synchronously with the language still
  // old, and the windows would keep switching each other back.
  if (shared.preference_theme && shared.preference_theme !== settings.preference_theme)
    settings.set_preference_theme(shared.preference_theme)

  if (shared.theme && shared.theme !== settings.theme)
    settings.set_theme(shared.theme)

  if (shared.language && shared.language !== gettext.current) {
    settings.set_language(shared.language)
    await Promise.all([
      loadTranslations(router.currentRoute.value),
      loadDayjsLocale(shared.language),
    ])
  }
}

/**
 * Runs inside a workspace pane. Reports focus, node and route to the shell
 * and follows the theme and language chosen in the workspace top bar.
 */
export function installPaneBridge(tabId: number, router: Router) {
  const settings = useSettingsStore()

  // Clicks inside an iframe never reach the parent document.
  window.addEventListener('pointerdown', () => postToShell(tabId, { type: 'focus' }), { capture: true })
  window.addEventListener('focus', () => postToShell(tabId, { type: 'focus' }))

  watch(
    () => ({ id: settings.node.id, name: settings.node.name }),
    node => postToShell(tabId, { type: 'node-changed', node }),
    { immediate: true },
  )

  // The title getter is reactive to the language and loaded translations, so
  // the tab label follows a language change too.
  void router.isReady().then(() => {
    watch(
      () => {
        const route = router.currentRoute.value
        return {
          path: route.fullPath,
          title: route.meta.name?.() ?? '',
          noAuth: !!route.meta.noAuth,
        }
      },
      ({ path, title, noAuth }) => {
        // A login or error page is not a place to come back to.
        if (!noAuth)
          postToShell(tabId, { type: 'route-changed', path, title })
      },
      { immediate: true },
    )
  })

  window.addEventListener('storage', event => {
    if (event.storageArea === localStorage && event.key === SETTINGS_STORAGE_KEY)
      void applySharedSettings(event.newValue, router)
  })

  // The pane without focus greys its header, like an inactive window.
  window.addEventListener('message', event => {
    if (event.source !== window.parent || event.origin !== location.origin || !isShellEnvelope(event.data))
      return

    document.documentElement.classList.toggle(INACTIVE_PANE_CLASS, !event.data.message.focused)
  })

  // The iframe's load event can fire before this code runs, so the shell
  // waits for this before telling the pane its focus state.
  postToShell(tabId, { type: 'ready' })

  // The language selector that normally does this is not shown in panes.
  void loadDayjsLocale(gettext.current)
}

// --------------------------------------------------------------- shell side

/** Sends a message to one pane window. */
export function postToPane(pane: Window, message: ShellMessage) {
  pane.postMessage(createShellEnvelope(message), location.origin)
}

function windowName(source: MessageEvent['source']): string | null {
  try {
    return source && 'name' in source ? (source as Window).name : null
  }
  catch {
    return null
  }
}

/** Receives pane messages. Returns a function that stops listening. */
export function listenToPanes(handler: (tabId: number, message: PaneMessage) => void): () => void {
  function onMessage(event: MessageEvent) {
    if (event.origin !== location.origin || !isPaneEnvelope(event.data))
      return

    // The envelope must come from the pane window it names.
    if (tabIdFromWindowName(windowName(event.source)) !== event.data.tabId)
      return

    handler(event.data.tabId, event.data.message)
  }

  window.addEventListener('message', onMessage)
  return () => window.removeEventListener('message', onMessage)
}

/**
 * Calls `onLogout` when the shared login token is cleared by any pane
 * (logout, session expiry, a rejected token). Returns a function that stops
 * listening.
 */
export function watchSharedLogout(onLogout: () => void): () => void {
  function onStorage(event: StorageEvent) {
    if (event.storageArea !== localStorage)
      return

    // `key` is null when the whole storage was cleared.
    if (event.key !== USER_STORAGE_KEY && event.key !== null)
      return

    if (!hasStoredToken(localStorage.getItem(USER_STORAGE_KEY)))
      onLogout()
  }

  window.addEventListener('storage', onStorage)
  return () => window.removeEventListener('storage', onStorage)
}
