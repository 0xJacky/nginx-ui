import type { Editor } from 'ace-builds'
import type { Point } from 'ace-builds-internal/document'
import type ReconnectingWebSocket from 'reconnecting-websocket'
import { debounce } from 'lodash'
import { v4 as uuidv4 } from 'uuid'
import openai from '@/api/openai'

// eslint-disable-next-line ts/no-explicit-any
function debug(...args: any[]) {
  if (import.meta.env.DEV) {
    // eslint-disable-next-line no-console
    console.debug(`[CodeEditor]`, ...args)
  }
}

// Config file patterns and extensions
const CONFIG_FILE_EXTENSIONS = ['.conf', '.config']
const SENSITIVE_CONTENT_PATTERNS = [
  /-----BEGIN [A-Z ]+ PRIVATE KEY-----/,
  /-----BEGIN CERTIFICATE-----/,
  /apiKey\s*[:=]\s*["'][a-zA-Z0-9]+["']/,
  /password\s*[:=]\s*["'][^"']+["']/,
  /secret\s*[:=]\s*["'][^"']+["']/,
]

function useCodeCompletion() {
  const editorRef = ref<Editor>()
  const currentGhostText = ref<string>('')
  const isConfigFile = ref<boolean>(false)

  const ws = shallowRef<ReconnectingWebSocket>()

  // Check if the current file is a configuration file
  function checkIfConfigFile(filename: string, content: string): boolean {
    // Check file extension
    const hasConfigExtension = CONFIG_FILE_EXTENSIONS.some(ext => filename.toLowerCase().endsWith(ext))

    // Check if it's an Nginx configuration file based on common patterns
    const hasNginxPatterns = /server\s*\{|location\s*\/|http\s*\{|upstream\s*[\w-]+\s*\{/.test(content)

    return hasConfigExtension || hasNginxPatterns
  }

  // Check if content contains sensitive information that shouldn't be sent
  function containsSensitiveContent(content: string): boolean {
    return SENSITIVE_CONTENT_PATTERNS.some(pattern => pattern.test(content))
  }

  function getAISuggestions(code: string, context: string, position: Point, callback: (suggestion: string) => void, language: string = 'nginx', suffix: string = '', requestId: string) {
    if (!ws.value || ws.value.readyState !== WebSocket.OPEN) {
      debug('WebSocket is not open')
      return
    }

    if (!code.trim()) {
      debug('Code is empty')
      return
    }

    // Skip if not a config file or contains sensitive content
    if (!isConfigFile.value) {
      debug('Skipping AI suggestions for non-config file')
      return
    }

    if (containsSensitiveContent(context)) {
      debug('Skipping AI suggestions due to sensitive content')
      return
    }

    const message = {
      context,
      code,
      suffix,
      language,
      position,
      request_id: requestId,
    }

    debug('Sending message', message)

    ws.value.send(JSON.stringify(message))

    ws.value.onmessage = event => {
      const data = JSON.parse(event.data)
      debug(`Received message`, data, requestId)
      if (data.request_id === requestId) {
        callback(data.code)
      }
    }
  }

  function applyGhostText() {
    if (!editorRef.value) {
      debug('Editor instance not available yet')
      return
    }

    if (!isConfigFile.value) {
      debug('Skipping ghost text for non-config file')
      return
    }

    try {
      const currentText = editorRef.value.getValue()

      // Skip if content contains sensitive information
      if (containsSensitiveContent(currentText)) {
        debug('Skipping ghost text due to sensitive content')
        return
      }

      const cursorPosition = editorRef.value.getCursorPosition()

      // Get all text before the current cursor position as the code part for the request
      const allLines = currentText.split('\n')
      const currentLine = allLines[cursorPosition.row]
      const textUpToCursor = allLines.slice(0, cursorPosition.row).join('\n')
        + (cursorPosition.row > 0 ? '\n' : '')
        + currentLine.substring(0, cursorPosition.column)

      // Get text after cursor position as suffix
      const textAfterCursor = currentLine.substring(cursorPosition.column)
        + (cursorPosition.row < allLines.length - 1 ? '\n' : '')
        + allLines.slice(cursorPosition.row + 1).join('\n')

      // Generate new request ID
      const requestId = uuidv4()

      // Clear existing ghost text before making the request
      clearGhostText()

      // Get AI suggestions
      getAISuggestions(
        textUpToCursor,
        currentText,
        cursorPosition,
        suggestion => {
          debug(`AI suggestions applied: ${suggestion}`)

          // If there's a suggestion, set ghost text
          if (suggestion && typeof editorRef.value!.setGhostText === 'function') {
            clearGhostText()

            // Get current cursor position (may have changed during async process)
            const newPosition = editorRef.value!.getCursorPosition()

            editorRef.value!.setGhostText(suggestion, {
              column: newPosition.column,
              row: newPosition.row,
            })
            debug(`Ghost text set: ${suggestion}`)
            currentGhostText.value = suggestion
          }
          else if (suggestion) {
            debug('setGhostText method not available on editor instance')
          }
        },
        editorRef.value.session.getMode()?.path?.split('/').pop() || 'text',
        textAfterCursor, // Pass text after cursor as suffix
        requestId, // Pass request ID
      )
    }
    catch (error) {
      debug(`Error in applyGhostText: ${error}`)
    }
  }

  // Accept the ghost text suggestion with Tab key
  function setupTabHandler(editor: Editor) {
    if (!editor) {
      debug('Editor not available in setupTabHandler')
      return
    }

    debug('Setting up Tab key handler')

    // Remove existing command to avoid conflicts
    const existingCommand = editor.commands.byName.acceptGhostText
    if (existingCommand) {
      editor.commands.removeCommand(existingCommand)
    }

    // Register new Tab key handler command with highest priority
    editor.commands.addCommand({
      name: 'acceptGhostText',
      bindKey: { win: 'Tab', mac: 'Tab' },
      exec: (editor: Editor) => {
        // Use our saved ghost text, not dependent on editor.ghostText
        if (currentGhostText.value) {
          debug(`Accepting ghost text: ${currentGhostText.value}`)

          const position = editor.getCursorPosition()
          const text = currentGhostText.value

          // Insert text through session API
          editor.session.insert(position, text)

          clearGhostText()

          debug('Ghost text inserted successfully')
          return true // Prevent event propagation
        }

        debug('No ghost text to accept, allowing default tab behavior')
        return false // Allow default Tab behavior
      },
      readOnly: false,
    })

    debug('Tab key handler set up successfully')
  }

  // Clear ghost text and reset state
  function clearGhostText() {
    if (!editorRef.value)
      return

    if (typeof editorRef.value.removeGhostText === 'function') {
      editorRef.value.removeGhostText()
    }
    currentGhostText.value = ''
  }

  const debouncedApplyGhostText = debounce(applyGhostText, 1000, { leading: false, trailing: true })

  debug('Editor initialized')

  async function init(editor: Editor, filename: string = '') {
    const { enabled } = await openai.get_code_completion_enabled_status()
    if (!enabled) {
      debug('Code completion is not enabled')
      return
    }

    ws.value = openai.code_completion()

    editorRef.value = editor

    // Determine if the current file is a configuration file
    const content = editor.getValue()
    isConfigFile.value = checkIfConfigFile(filename, content)
    debug(`File type check: isConfigFile=${isConfigFile.value}, filename=${filename}`)

    // Set up Tab key handler
    setupTabHandler(editor)

    setTimeout(() => {
      editor.on('change', (e: { action: string }) => {
        debug(`Editor change event: ${e.action}`)
        // If change is caused by user input, interrupt current completion
        clearGhostText()

        if (e.action === 'insert' || e.action === 'remove') {
          // Clear current ghost text
          if (isConfigFile.value) {
            debouncedApplyGhostText()
          }
        }
      })

      // Listen for cursor changes, using debounce
      editor.selection.on('changeCursor', () => {
        debug('Cursor changed')
        clearGhostText()
        if (isConfigFile.value) {
          debouncedApplyGhostText()
        }
      })
    }, 2000)
  }

  function cleanUp() {
    if (ws.value) {
      ws.value.close()
    }
    debug('CodeCompletion unmounted')
  }

  return {
    init,
    cleanUp,
  }
}

export default useCodeCompletion
