import type { Editor } from 'ace-builds'
import type { Point } from 'ace-builds-internal/document'
import ace from 'ace-builds'
import { debounce } from 'lodash'
import { v4 as uuidv4 } from 'uuid'
import llm from '@/api/llm'
import { useWebSocket } from '@/lib/websocket'

function debug(...args: unknown[]) {
  if (import.meta.env.DEV) {
    // Create console method that skips one frame in stack trace
    // eslint-disable-next-line no-console
    const originalDebug = console.debug
    const skipFrame = (...logArgs: unknown[]) => {
      const stack = new Error('Debug trace').stack
      const caller = stack?.split('\n')[3] // Skip debug() and skipFrame()
      const match = caller?.match(/at\s[^(]+\([^:]+:(\d+):\d+\)/) || caller?.match(/at\s[^:]+:(\d+):\d+/)
      const location = `line ${match?.[1] || '?'}`

      originalDebug(`[CodeEditor:${location}]`, ...logArgs)
    }

    skipFrame(...args)
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
  const isLoading = ref<boolean>(false)
  const loadingMarkerId = ref<number | null>(null)
  const lastTriggerTime = ref<number>(0)
  const lastTriggerPosition = ref<{ row: number, column: number } | null>(null)

  const ws = shallowRef<WebSocket>()

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

  // Show loading spinner at cursor position
  function showLoadingSpinner() {
    if (!editorRef.value || isLoading.value)
      return

    const position = editorRef.value.getCursorPosition()
    const Range = ace.require('ace/range').Range

    // Create a small range at cursor position for the spinner
    const range = new Range(position.row, position.column, position.row, position.column + 1)

    loadingMarkerId.value = editorRef.value.session.addMarker(
      range,
      'completion-loading-spinner',
      'text',
      false,
    )

    isLoading.value = true
    debug('Loading spinner shown')
  }

  // Clear loading spinner
  function clearLoadingSpinner() {
    if (!editorRef.value || !isLoading.value)
      return

    if (loadingMarkerId.value !== null) {
      editorRef.value.session.removeMarker(loadingMarkerId.value)
      loadingMarkerId.value = null
    }

    isLoading.value = false
    debug('Loading spinner cleared')
  }

  // Get current line indentation
  function getCurrentLineIndent(editor: Editor): string {
    const position = editor.getCursorPosition()
    const currentLine = editor.session.getLine(position.row)

    // Extract indentation (spaces/tabs at the beginning of line)
    const indentMatch = currentLine.match(/^(\s*)/)
    return indentMatch ? indentMatch[1] : ''
  }

  // Intelligent trigger condition checking
  function checkRateLimit(): boolean {
    const currentTime = Date.now()
    if (currentTime - lastTriggerTime.value < 800) {
      debug(`Skipping completion: too frequent triggers. Time diff: ${currentTime - lastTriggerTime.value}ms`)
      return false
    }
    return true
  }

  function checkPositionLimit(position: Point): boolean {
    if (lastTriggerPosition.value) {
      const rowDiff = Math.abs(position.row - lastTriggerPosition.value.row)
      const colDiff = Math.abs(position.column - lastTriggerPosition.value.column)

      if (rowDiff === 0 && colDiff < 4) {
        debug(`Skipping completion: cursor position too close to last trigger. Row diff: ${rowDiff}, Col diff: ${colDiff}`)
        return false
      }
    }
    return true
  }

  function updateTriggerTracking(position: Point): void {
    const currentTime = Date.now()
    lastTriggerTime.value = currentTime
    lastTriggerPosition.value = { row: position.row, column: position.column }
  }

  function checkShortLineContext(editor: Editor, position: Point, beforeCursor: string): boolean {
    if (beforeCursor.trim().length >= 2)
      return true

    const isEmptyLineInBlock = /^\s+$/.test(beforeCursor) && beforeCursor.length > 0
    const prevLine = position.row > 0 ? editor.session.getLine(position.row - 1) : ''
    const afterComment = /^\s*[#/]/.test(prevLine)

    if (!isEmptyLineInBlock && !afterComment) {
      debug('Skipping completion: line too short and not in meaningful context')
      return false
    }

    if (isEmptyLineInBlock || afterComment) {
      debug('Allowing completion: empty line in block or after comment')
      updateTriggerTracking(position)
      return true
    }

    return false
  }

  function checkWordBoundary(beforeCursor: string, afterCursor: string): boolean {
    if (afterCursor.match(/^\w/) && !beforeCursor.endsWith('{')) {
      debug('Skipping completion: cursor in middle of word')
      return false
    }
    return true
  }

  function checkCommentLine(currentLine: string, position: Point): boolean {
    const isCommentLine = /^\s*[#/]/.test(currentLine)
    if (isCommentLine) {
      const commentContent = currentLine.replace(/^\s*[#/]+\s*/, '')
      const hasDirectivePattern = /\b(?:proxy_pass|server_name|root|listen|location)\b/.test(commentContent)

      if (!hasDirectivePattern || position.column < currentLine.length - 1) {
        debug('Skipping completion: comment line without directive pattern or not at end')
        return false
      }
    }
    return true
  }

  function checkLineCompletion(currentLine: string, position: Point): boolean {
    const trimmedLine = currentLine.trim()
    const atLineEnd = position.column === currentLine.length

    if (trimmedLine.endsWith(';') || (trimmedLine.endsWith('}') && !atLineEnd)) {
      debug('Skipping completion: line appears complete')
      return false
    }
    return true
  }

  function shouldTriggerCompletion(editor: Editor): boolean {
    if (!editor)
      return false

    const position = editor.getCursorPosition()

    if (!checkRateLimit() || !checkPositionLimit(position)) {
      return false
    }

    const currentLine = editor.session.getLine(position.row)
    const beforeCursor = currentLine.substring(0, position.column)
    const afterCursor = currentLine.substring(position.column)

    const shortLineResult = checkShortLineContext(editor, position, beforeCursor)
    if (beforeCursor.trim().length < 2) {
      return shortLineResult
    }

    if (!checkWordBoundary(beforeCursor, afterCursor) || !checkCommentLine(currentLine, position) || !checkLineCompletion(currentLine, position)) {
      return false
    }

    const trimmedLine = currentLine.trim()
    const atLineEnd = position.column === currentLine.length

    if (trimmedLine.endsWith('{') && atLineEnd) {
      debug('Allowing completion after opening brace at line end')
      return true
    }

    if (/["'[(]\s*$/.test(beforeCursor)) {
      debug('Skipping completion: cursor after quote/bracket')
      return false
    }

    if (beforeCursor.endsWith('  ')) {
      debug('Skipping completion: multiple spaces detected')
      return false
    }

    // Check each trigger pattern individually for better debugging
    const triggerPatterns = [
      { name: 'After directive keywords', pattern: /\b(?:server|location|upstream|if|proxy_pass|root|index|listen|server_name)\s+$/ },
      { name: 'Right after opening brace', pattern: /\{\s*$/ },
      { name: 'After semicolons', pattern: /;\s*$/ },
      { name: 'Partial directive names', pattern: /^\s+[a-z_]{3,}$/i },
      { name: 'Comment with directive', pattern: /^\s*#.*\b(?:proxy_pass|server_name|root|listen|location)\b/ },
      { name: 'Empty line in block', pattern: /^\s+$/ },
    ]

    let shouldTrigger = false

    for (const { name, pattern } of triggerPatterns) {
      if (pattern.test(beforeCursor)) {
        shouldTrigger = true
        debug(`✅ Trigger match: ${name} | Line: "${currentLine}" | Pos: ${position.row}:${position.column}`)
        break
      }
    }

    if (shouldTrigger) {
      // Update trigger tracking
      updateTriggerTracking(position)
    }

    return shouldTrigger
  }

  function getAISuggestions(code: string, context: string, position: Point, callback: (suggestion: string) => void, language: string = 'nginx', suffix: string = '', requestId: string, currentIndent: string = '') {
    if (!ws.value || ws.value.readyState !== WebSocket.OPEN) {
      debug('WebSocket is not open')
      clearLoadingSpinner()
      return
    }

    if (!code.trim()) {
      debug('Code is empty')
      clearLoadingSpinner()
      return
    }

    // Skip if not a config file or contains sensitive content
    if (!isConfigFile.value) {
      debug('Skipping AI suggestions for non-config file')
      clearLoadingSpinner()
      return
    }

    if (containsSensitiveContent(context)) {
      debug('Skipping AI suggestions due to sensitive content')
      clearLoadingSpinner()
      return
    }

    const message = {
      context,
      code,
      suffix,
      language,
      position,
      request_id: requestId,
      current_indent: currentIndent,
    }

    debug('Sending message', message)

    // Show loading spinner when sending request
    showLoadingSpinner()

    ws.value.send(JSON.stringify(message))

    ws.value.onmessage = event => {
      const data = JSON.parse(event.data)
      debug(`Received message`, data, requestId)
      if (data.request_id === requestId) {
        // Clear loading spinner when receiving response
        clearLoadingSpinner()
        callback(data.code)
      }
    }
  }

  function applyGhostText() {
    if (!editorRef.value) {
      return
    }

    if (!isConfigFile.value) {
      return
    }

    // Intelligent trigger check
    if (!shouldTriggerCompletion(editorRef.value)) {
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

      // Get current line indentation for proper formatting
      const currentLineIndent = getCurrentLineIndent(editorRef.value)

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

            // Smart formatting: handle line-end completions
            const formattedSuggestion = formatCompletionForPosition(suggestion, newPosition)

            editorRef.value!.setGhostText(formattedSuggestion, {
              column: newPosition.column,
              row: newPosition.row,
            })
            debug(`Ghost text set: ${formattedSuggestion}`)
            currentGhostText.value = formattedSuggestion
          }
          else if (suggestion) {
            debug('setGhostText method not available on editor instance')
          }
        },
        editorRef.value.session.getMode()?.path?.split('/').pop() || 'text',
        textAfterCursor, // Pass text after cursor as suffix
        requestId, // Pass request ID
        currentLineIndent, // Pass current line indentation
      )
    }
    catch (error) {
      debug(`Error in applyGhostText: ${error}`)
    }
  }

  // Accept the ghost text suggestion with Tab key and clear with Esc key
  function setupKeyHandlers(editor: Editor) {
    if (!editor) {
      debug('Editor not available in setupKeyHandlers')
      return
    }

    debug('Setting up key handlers')

    // Remove existing commands to avoid conflicts
    const existingTabCommand = editor.commands.byName.acceptGhostText
    if (existingTabCommand) {
      editor.commands.removeCommand(existingTabCommand)
    }

    const existingEscCommand = editor.commands.byName.clearGhostText
    if (existingEscCommand) {
      editor.commands.removeCommand(existingEscCommand)
    }

    // Register Tab key handler - accept ghost text
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

    // Register Esc key handler - clear ghost text
    editor.commands.addCommand({
      name: 'clearGhostText',
      bindKey: { win: 'Escape', mac: 'Escape' },
      exec: (_editor: Editor) => {
        if (currentGhostText.value) {
          debug('Clearing ghost text with Esc key')
          clearGhostText()
          return true // Prevent event propagation
        }

        debug('No ghost text to clear, allowing default escape behavior')
        return false // Allow default Escape behavior
      },
      readOnly: false,
    })

    debug('Key handlers set up successfully')
  }

  // Clear ghost text and reset state
  function clearGhostText() {
    if (!editorRef.value)
      return

    if (typeof editorRef.value.removeGhostText === 'function') {
      editorRef.value.removeGhostText()
    }
    currentGhostText.value = ''

    // Also clear loading spinner
    clearLoadingSpinner()

    // Reset trigger tracking when manually clearing
    lastTriggerTime.value = 0
    lastTriggerPosition.value = null
  }

  const debouncedApplyGhostText = debounce(applyGhostText, 1000, { leading: false, trailing: true })

  debug('Editor initialized')

  async function init(editor: Editor, filename: string = '') {
    const { enabled } = await llm.get_code_completion_enabled_status()
    if (!enabled) {
      debug('Code completion is not enabled')
      return
    }

    const { ws: wsRef } = useWebSocket(llm.codeCompletionWebSocketUrl, false)
    ws.value = wsRef.value!

    editorRef.value = editor

    // Determine if the current file is a configuration file
    const content = editor.getValue()
    isConfigFile.value = checkIfConfigFile(filename, content)
    debug(`File type check: isConfigFile=${isConfigFile.value}, filename=${filename}`)

    // Set up key handlers (Tab and Esc)
    setupKeyHandlers(editor)

    setTimeout(() => {
      editor.on('change', (e: { action: string }) => {
        // If change is caused by user input, interrupt current completion
        clearGhostText()

        if ((e.action === 'insert' || e.action === 'remove') && isConfigFile.value) {
          debouncedApplyGhostText()
        }
      })

      // Listen for cursor changes, using debounce
      editor.selection.on('changeCursor', () => {
        clearGhostText()
        if (isConfigFile.value) {
          debouncedApplyGhostText()
        }
      })
    }, 2000)
  }

  function cleanUp() {
    clearLoadingSpinner()
    clearGhostText()
    if (ws.value) {
      ws.value.close()
    }
    debug('CodeCompletion unmounted')
  }

  // Smart formatting for completion suggestions
  function formatCompletionForPosition(suggestion: string, position: Point): string {
    if (!editorRef.value)
      return suggestion

    const currentLine = editorRef.value.session.getLine(position.row)
    const beforeCursor = currentLine.substring(0, position.column)

    // Check if cursor is at the end of a non-empty line
    const atEndOfLine = position.column === currentLine.length
    const lineHasContent = beforeCursor.trim().length > 0

    // If at end of line with content, and suggestion doesn't start with newline
    if (atEndOfLine && lineHasContent && !suggestion.startsWith('\n')) {
      // Check if suggestion should be on new line (block syntax, etc.)
      const shouldNewline = shouldStartOnNewLine(suggestion, beforeCursor)
      if (shouldNewline) {
        const indent = getIndentForNewLine(beforeCursor)
        return `\n${indent}${suggestion}`
      }
    }

    // If suggestion starts with newline but we're not at line end, remove it
    if (!atEndOfLine && suggestion.startsWith('\n')) {
      return suggestion.substring(1)
    }

    return suggestion
  }

  // Determine if completion should start on new line
  function shouldStartOnNewLine(suggestion: string, beforeCursor: string): boolean {
    // Nginx block patterns that usually need newlines
    const blockPatterns = [
      /^\s*server\s*\{/, // server block
      /^\s*location\s*(?:\S.*(?:[\n\r\u2028\u2029]\s*)?)?\{/, // location block
      /^\s*upstream\s*\w+\s*\{/, // upstream block
      /^\s*if\s*\(.*\)\s*\{/, // if block
    ]

    const directivePatterns = [
      /^\s*(listen|server_name|root|index|location|proxy_pass|return)/,
      /^\s*(error_page|access_log|error_log|ssl_certificate)/,
    ]

    // If before cursor ends with { or ;, next content should be on new line
    if (/[{;]\s*$/.test(beforeCursor)) {
      return true
    }

    // If suggestion looks like a new directive/block
    const isBlockSuggestion = blockPatterns.some(pattern => pattern.test(suggestion))
    const isDirectiveSuggestion = directivePatterns.some(pattern => pattern.test(suggestion))

    return isBlockSuggestion || isDirectiveSuggestion
  }

  // Get appropriate indentation for new line
  function getIndentForNewLine(beforeCursor: string): string {
    const baseIndent = beforeCursor.match(/^\s*/)?.[0] || ''

    // If previous line ends with {, increase indentation
    if (beforeCursor.trim().endsWith('{')) {
      return `${baseIndent}    ` // 4 spaces
    }

    return baseIndent
  }

  return {
    init,
    cleanUp,
  }
}

export default useCodeCompletion
