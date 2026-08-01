import type { TerminalSessionCallbacks } from '@/composables/useTerminalSession'
import type { TerminalTab } from '@/pinia/moudule/terminal'
import { FitAddon } from '@xterm/addon-fit'
import { Terminal } from '@xterm/xterm'
import { throttle } from 'lodash'

/**
 * A terminal that never leaves the browser.
 *
 * The demo instance refuses /api/pty outright, so there is no PTY to attach to.
 * Rather than showing a dead panel, this drives xterm from a local command
 * table. Two things fall out of that: nothing can be executed on the host, and
 * no WebSocket stays open — which matters on Cloudflare Containers, where an
 * in-flight request keeps the container from ever going to sleep.
 */

const PROMPT = '\x1B[1;32mdemo@nginx-ui\x1B[0m:\x1B[1;34m~\x1B[0m$ '

interface DemoCommand {
  /** Lines written back, without the trailing prompt. */
  output: string[] | (() => string[])
  help: string
}

function table(rows: string[][], gap = 2): string[] {
  const widths = rows[0].map((_, i) => Math.max(...rows.map(r => (r[i] ?? '').length)))
  return rows.map(r =>
    r.map((cell, i) => (i === r.length - 1 ? cell : (cell ?? '').padEnd(widths[i] + gap))).join(''),
  )
}

const COMMANDS: Record<string, DemoCommand> = {
  'whoami': { help: 'print the current user', output: ['nginx'] },
  'id': { help: 'print user identity', output: ['uid=101(nginx) gid=101(nginx) groups=101(nginx)'] },
  'hostname': { help: 'print the node name', output: ['nginx-ui-demo'] },
  'pwd': { help: 'print the working directory', output: ['/'] },
  'date': { help: 'print the current time', output: () => [new Date().toUTCString()] },
  'uptime': {
    help: 'show how long the node has been up',
    output: () => {
      const up = Math.floor(performance.now() / 1000)
      const h = String(Math.floor(up / 3600)).padStart(2, '0')
      const m = String(Math.floor((up % 3600) / 60)).padStart(2, '0')
      return [` ${new Date().toTimeString().slice(0, 8)} up ${h}:${m},  1 user,  load average: 0.08, 0.11, 0.09`]
    },
  },
  'uname -a': {
    help: 'print kernel information',
    output: ['Linux nginx-ui-demo 6.6.0 #1 SMP x86_64 GNU/Linux'],
  },
  'free -h': {
    help: 'show memory usage',
    output: () => table([
      ['', 'total', 'used', 'free', 'shared', 'buff/cache', 'available'],
      ['Mem:', '1.0Gi', '537Mi', '213Mi', '0.0Ki', '284Mi', '463Mi'],
      ['Swap:', '0B', '0B', '0B', '', '', ''],
    ]),
  },
  'df -h': {
    help: 'show disk usage',
    output: () => table([
      ['Filesystem', 'Size', 'Used', 'Avail', 'Use%', 'Mounted on'],
      ['overlay', '4.0G', '712M', '3.3G', '18%', '/'],
      ['tmpfs', '64M', '0', '64M', '0%', '/dev'],
    ]),
  },
  'ls': { help: 'list /etc/nginx', output: ['conf.d  mime.types  nginx.conf  sites-available  sites-enabled  streams-available  streams-enabled'] },
  'ls -la': {
    help: 'list /etc/nginx in detail',
    output: () => table([
      ['drwxr-xr-x', '1', 'nginx', 'nginx', '4096', 'conf.d'],
      ['-rw-r--r--', '1', 'nginx', 'nginx', '5349', 'mime.types'],
      ['-rw-r--r--', '1', 'nginx', 'nginx', '1042', 'nginx.conf'],
      ['drwxr-xr-x', '1', 'nginx', 'nginx', '4096', 'sites-available'],
      ['drwxr-xr-x', '1', 'nginx', 'nginx', '4096', 'sites-enabled'],
    ]),
  },
  'cat /etc/os-release': {
    help: 'print the distribution',
    output: [
      'PRETTY_NAME="Debian GNU/Linux 13 (trixie)"',
      'NAME="Debian GNU/Linux"',
      'VERSION_ID="13"',
      'ID=debian',
    ],
  },
  'nginx -v': { help: 'print the nginx version', output: ['nginx version: nginx/1.31.3'] },
  'nginx -V': {
    help: 'print the nginx build configuration',
    output: [
      'nginx version: nginx/1.31.3',
      'built by gcc 14.2.0 (Debian 14.2.0-19)',
      'built with OpenSSL 3.5.4',
      'TLS SNI support enabled',
      'configure arguments: --prefix=/etc/nginx --with-http_ssl_module --with-http_v2_module --with-stream',
    ],
  },
  'nginx -t': {
    help: 'test the nginx configuration',
    output: [
      'nginx: the configuration file /etc/nginx/nginx.conf syntax is ok',
      'nginx: configuration file /etc/nginx/nginx.conf test is successful',
    ],
  },
}

const MAX_INPUT = 256
const MAX_HISTORY = 50

export interface DemoTerminalSession {
  tab: TerminalTab
  terminal: Terminal
  fitAddon: FitAddon
  dispose: () => void
}

function banner(): string[] {
  return [
    '\x1B[1mNginx UI demo shell\x1B[0m',
    '',
    'This terminal is simulated in your browser. No command reaches a real host,',
    'and the demo instance does not expose a PTY endpoint at all.',
    '',
    `Try: \x1B[36m${Object.keys(COMMANDS).slice(0, 6).join('\x1B[0m, \x1B[36m')}\x1B[0m`,
    'Type \x1B[36mhelp\x1B[0m for the full list.',
    '',
  ]
}

function helpText(): string[] {
  const rows = Object.entries(COMMANDS).map(([name, cmd]) => [name, cmd.help])
  rows.push(['help', 'show this list'])
  rows.push(['clear', 'clear the screen'])
  return ['Available commands:', '', ...table(rows, 4).map(line => `  ${line}`), '']
}

export function useDemoTerminalSession() {
  const sessions = new Map<string, DemoTerminalSession>()

  function createSession(
    tab: TerminalTab,
    containerId: string,
    callbacks?: TerminalSessionCallbacks,
  ): DemoTerminalSession {
    const terminal = new Terminal({
      convertEol: true,
      fontSize: 14,
      cursorStyle: 'block',
      scrollback: 1000,
      theme: { background: '#000' },
    })

    const fitAddon = new FitAddon()
    terminal.loadAddon(fitAddon)

    const element = document.getElementById(containerId)
    if (element) {
      terminal.open(element)
      fitAddon.fit()
    }

    let buffer = ''
    const history: string[] = []
    let historyCursor = -1

    const writeLines = (lines: string[]) => lines.forEach(line => terminal.writeln(line))
    const prompt = () => terminal.write(PROMPT)

    const replaceInput = (next: string) => {
      terminal.write(`\r\x1B[K${PROMPT}${next}`)
      buffer = next
    }

    const run = (raw: string) => {
      const command = raw.trim()
      if (!command) {
        return
      }

      history.unshift(command)
      history.splice(MAX_HISTORY)
      callbacks?.onInput?.(tab.id, `${command}\r`)

      if (command === 'clear') {
        terminal.clear()
        return
      }
      if (command === 'help') {
        writeLines(helpText())
        return
      }

      const entry = COMMANDS[command]
      if (!entry) {
        writeLines([
          `\x1B[31m${command.split(' ')[0]}: command not found in the demo shell\x1B[0m`,
          'Type \x1B[36mhelp\x1B[0m to see what is available.',
        ])
        return
      }

      writeLines(typeof entry.output === 'function' ? entry.output() : entry.output)
    }

    const submit = () => {
      terminal.write('\r\n')
      run(buffer)
      buffer = ''
      historyCursor = -1
      prompt()
    }

    const onData = terminal.onData(data => {
      // Whole-chunk matches first. These arrive as a single string and would be
      // meaningless if split into characters below.
      switch (data) {
        case '\u001B[A': // Up
          if (historyCursor + 1 < history.length) {
            historyCursor += 1
            replaceInput(history[historyCursor])
          }
          return
        case '\u001B[B': // Down
          if (historyCursor > 0) {
            historyCursor -= 1
            replaceInput(history[historyCursor])
          }
          else if (historyCursor === 0) {
            historyCursor = -1
            replaceInput('')
          }
          return
      }

      // Everything else is walked per character, so a paste spanning several
      // lines runs each line instead of landing in the buffer verbatim.
      for (const char of data) {
        switch (char) {
          // Enter arrives as \r from a keypress and \n from pasted input.
          case '\r':
          case '\n':
            submit()
            break
          case '\u007F': // Backspace
            if (buffer.length > 0) {
              buffer = buffer.slice(0, -1)
              terminal.write('\b \b')
            }
            break
          case '\u0003': // Ctrl+C
            terminal.write('^C\r\n')
            buffer = ''
            historyCursor = -1
            prompt()
            break
          case '\u000C': // Ctrl+L
            terminal.clear()
            replaceInput(buffer)
            break
          default:
            // Printable only, so stray escape sequences cannot corrupt the buffer.
            if (char >= ' ' && buffer.length < MAX_INPUT) {
              buffer += char
              terminal.write(char)
            }
        }
      }
    })

    writeLines(banner())
    prompt()
    callbacks?.onConnectionReady?.(tab.id)

    const fit = throttle(() => fitAddon.fit(), 50)
    window.addEventListener('resize', fit)

    const session: DemoTerminalSession = {
      tab,
      terminal,
      fitAddon,
      dispose: () => {
        window.removeEventListener('resize', fit)
        onData.dispose()
        terminal.dispose()
        sessions.delete(tab.id)
      },
    }

    sessions.set(tab.id, session)
    return session
  }

  const destroySession = (tabId: string) => sessions.get(tabId)?.dispose()
  const focusSession = (tabId: string) => sessions.get(tabId)?.terminal.focus()
  const resizeAllSessions = () => sessions.forEach(session => session.fitAddon.fit())

  return {
    createSession,
    destroySession,
    focusSession,
    resizeAllSessions,
    /** Commands the fake shell answers — exported so tests can assert coverage. */
    supportedCommands: Object.keys(COMMANDS),
  }
}
