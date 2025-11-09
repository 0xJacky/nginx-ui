import type { TerminalTab } from '@/pinia/moudule/terminal'
import { FitAddon } from '@xterm/addon-fit'
import { Terminal } from '@xterm/xterm'
import { throttle } from 'lodash'
import { useWebSocket } from '@/lib/websocket'

export interface TerminalSession {
  tab: TerminalTab
  terminal: Terminal
  websocket: WebSocket
  fitAddon: FitAddon
  ping?: ReturnType<typeof setTimeout>
  isWebSocketReady: boolean
  lostConnection: boolean
}

export interface TerminalSessionCallbacks {
  onInput?: (tabId: string, data: string) => void
  onConnectionLost?: (tabId: string) => void
  onConnectionReady?: (tabId: string) => void
}

interface Message {
  Type: number
  Data: string | null | { Cols: number, Rows: number }
}

export function useTerminalSession() {
  const sessions = new Map<string, TerminalSession>()

  const createSession = async (
    tab: TerminalTab,
    containerId: string,
    secureSessionId: string,
    callbacks?: TerminalSessionCallbacks,
  ): Promise<TerminalSession> => {
    const terminal = new Terminal({
      convertEol: true,
      fontSize: 14,
      cursorStyle: 'block',
      scrollback: 1000,
      theme: {
        background: '#000',
      },
    })

    const fitAddon = new FitAddon()
    terminal.loadAddon(fitAddon)

    const { ws } = useWebSocket(`/api/pty?X-Secure-Session-ID=${secureSessionId}`, false)
    const websocket = ws.value!

    const session: TerminalSession = {
      tab,
      terminal,
      websocket,
      fitAddon,
      isWebSocketReady: false,
      lostConnection: false,
    }

    const fit = throttle(() => {
      fitAddon.fit()
    }, 50)

    const sendMessage = (data: Message) => {
      if (session.websocket && session.isWebSocketReady) {
        session.websocket.send(JSON.stringify(data))
      }
    }

    const wsOnMessage = (msg: { data: string | Uint8Array }) => {
      terminal.write(msg.data)
    }

    const wsOnOpen = () => {
      session.isWebSocketReady = true
      session.lostConnection = false
      session.ping = setInterval(() => {
        sendMessage({ Type: 3, Data: null })
      }, 30000)
      callbacks?.onConnectionReady?.(tab.id)
    }

    const handleConnectionLost = () => {
      session.lostConnection = true
      session.isWebSocketReady = false
      callbacks?.onConnectionLost?.(tab.id)
    }

    const wsOnError = handleConnectionLost
    const wsOnClose = handleConnectionLost

    websocket.onmessage = wsOnMessage
    websocket.onopen = wsOnOpen
    websocket.onerror = wsOnError
    websocket.onclose = wsOnClose

    terminal.onData(key => {
      const order: Message = {
        Data: key,
        Type: 1,
      }

      callbacks?.onInput?.(tab.id, key)
      sendMessage(order)
    })

    terminal.onBinary(data => {
      sendMessage({ Type: 1, Data: data })
    })

    terminal.onResize(data => {
      sendMessage({ Type: 2, Data: { Cols: data.cols, Rows: data.rows } })
    })

    const container = document.getElementById(containerId)
    if (!container) {
      throw new Error(`Terminal container with id "${containerId}" not found`)
    }

    terminal.open(container)

    setTimeout(() => {
      fitAddon.fit()
    }, 60)

    window.addEventListener('resize', fit)
    terminal.focus()

    sessions.set(tab.id, session)
    return session
  }

  const destroySession = (tabId: string) => {
    const session = sessions.get(tabId)
    if (!session)
      return

    clearInterval(session.ping)
    session.terminal.dispose()
    session.websocket.close()
    sessions.delete(tabId)
  }

  const getSession = (tabId: string): TerminalSession | undefined => {
    return sessions.get(tabId)
  }

  const focusSession = (tabId: string) => {
    const session = sessions.get(tabId)
    if (session) {
      session.terminal.focus()
      setTimeout(() => {
        session.fitAddon.fit()
      }, 100)
    }
  }

  const resizeSession = (tabId: string) => {
    const session = sessions.get(tabId)
    if (session) {
      setTimeout(() => {
        session.fitAddon.fit()
      }, 100)
    }
  }

  const resizeAllSessions = () => {
    sessions.forEach(session => {
      setTimeout(() => {
        session.fitAddon.fit()
      }, 100)
    })
  }

  const getSessionConnectionStatus = (tabId: string) => {
    const session = sessions.get(tabId)
    if (!session) {
      return { isReady: false, lostConnection: false }
    }
    return {
      isReady: session.isWebSocketReady,
      lostConnection: session.lostConnection,
    }
  }

  const hasAnyConnectionLoss = computed(() => {
    return Array.from(sessions.values()).some(session => session.lostConnection)
  })

  return {
    createSession,
    destroySession,
    getSession,
    focusSession,
    resizeSession,
    resizeAllSessions,
    getSessionConnectionStatus,
    hasAnyConnectionLoss,
    sessions: readonly(sessions),
  }
}
