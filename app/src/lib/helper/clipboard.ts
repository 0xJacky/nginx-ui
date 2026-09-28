/**
 * Copies text to the clipboard and rejects when the copy did not happen.
 *
 * The async Clipboard API only exists in secure contexts, so plain-HTTP
 * deployments fall back to `document.execCommand('copy')`. The hidden
 * textarea is mounted next to the focused element so modal focus traps
 * do not pull focus (and the selection) away before the copy runs.
 */
async function copyText(text: string) {
  if (window.isSecureContext && navigator.clipboard?.writeText) {
    try {
      await navigator.clipboard.writeText(text)
      return
    }
    catch {
      // Permission denied or document not focused; try the legacy path.
    }
  }

  legacyCopy(text)
}

function legacyCopy(text: string) {
  const activeElement = document.activeElement as HTMLElement | null
  const container = activeElement?.closest('[role="dialog"]') ?? document.body
  const selection = document.getSelection()
  const previousRange = selection && selection.rangeCount > 0 ? selection.getRangeAt(0) : null

  const textarea = document.createElement('textarea')
  textarea.value = text
  textarea.setAttribute('readonly', '')
  textarea.style.position = 'fixed'
  textarea.style.top = '0'
  textarea.style.left = '0'
  textarea.style.opacity = '0'
  textarea.style.pointerEvents = 'none'
  container.appendChild(textarea)

  let isCopied = false
  try {
    textarea.focus({ preventScroll: true })
    textarea.select()
    textarea.setSelectionRange(0, text.length)
    isCopied = document.execCommand('copy')
  }
  finally {
    textarea.remove()
    activeElement?.focus?.({ preventScroll: true })
    if (previousRange && selection) {
      selection.removeAllRanges()
      selection.addRange(previousRange)
    }
  }

  if (!isCopied)
    throw new Error('Clipboard copy is not available in this browser context')
}

export { copyText }
