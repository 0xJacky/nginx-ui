import { tabIdFromWindowName } from './paneStorage'

/**
 * The workspace tab this window renders, when it is a workspace pane. Known at
 * boot from the iframe name, before any code runs in the pane.
 */
export const paneTabId: number | null = window.parent !== window ? tabIdFromWindowName(window.name) : null

/** This window is a pane inside the workspace rather than the main UI. */
export const isWorkspacePane = paneTabId !== null

// Kept for code that still reads the old flag.
window.inWorkspace = isWorkspacePane

// Lets global styles (the slim pane header) apply before the first render.
if (isWorkspacePane)
  document.documentElement.classList.add('workspace-pane')
