import { readFileSync } from 'node:fs'
import { fileURLToPath } from 'node:url'
import { describe, expect, it } from 'vitest'

describe('WebTerminal view source', () => {
  it('uses xterm.js interactive terminal with websocket input and resize messages', () => {
    const source = readFileSync(fileURLToPath(new URL('./WebTerminal.vue', import.meta.url)), 'utf8')

    expect(source).toContain("import { FitAddon } from '@xterm/addon-fit'")
    expect(source).toContain("import { Terminal } from '@xterm/xterm'")
    expect(source).toContain("import { onBeforeRouteLeave } from 'vue-router'")
    expect(source).not.toContain('useRouter')
    expect(source).toContain("import '@xterm/xterm/css/xterm.css'")
    expect(source).toContain("const normalizedBaseUrl = rawBaseUrl.endsWith('/') ? rawBaseUrl : `${rawBaseUrl}/`")
    expect(source).toContain("const wsUrl = new URL(`${normalizedBaseUrl}api/webssh/ws`, window.location.origin)")
    expect(source).toContain("type: 'input', data")
    expect(source).toContain("type: 'resize',")
    expect(source).toContain('cols: currentTerminal.cols')
    expect(source).toContain('rows: currentTerminal.rows')
    expect(source).toContain('const currentSocket = new WebSocket(wsUrl)')
  })

  it('fits terminal viewport and sends resize on open and host resize', () => {
    const source = readFileSync(fileURLToPath(new URL('./WebTerminal.vue', import.meta.url)), 'utf8')

    expect(source).toContain('fitAddon.value?.fit()')
    expect(source).toContain('sendResize()')
    expect(source).toContain('const observer = new ResizeObserver(() => {')
    expect(source).toContain("window.addEventListener('beforeunload', handleBeforeUnload)")
    expect(source).toContain('onBeforeRouteLeave((to, _from, next) => {')
    expect(source).toContain('proceed: () => next(),')
    expect(source).toContain('cancel: () => next(false),')
    expect(source).not.toContain('await router.push(targetPath)')
  })

  it('guards websocket frame parsing and tears down stale sockets safely on error and close', () => {
    const source = readFileSync(fileURLToPath(new URL('./WebTerminal.vue', import.meta.url)), 'utf8')

    expect(source).toContain('try {')
    expect(source).toContain('JSON.parse(String(event.data)) as WebTerminalServerMessage')
    expect(source).toContain("text: terminalText('terminalMalformedMessage')")
    expect(source).toContain("console.warn('[WebTerminal] ignored malformed websocket payload'")
    expect(source).toContain('const currentSocket = new WebSocket')
    expect(source).toContain('if (socket.value === currentSocket) {')
    expect(source).toContain('currentSocket.close()')
  })

  it('renders the terminal status, transcript, command input, and connect controls', () => {
    const source = readFileSync(fileURLToPath(new URL('./WebTerminal.vue', import.meta.url)), 'utf8')

    expect(source).toContain("$t('webTerminal.connectionStatus')")
    expect(source).toContain("$t('webTerminal.transcript')")
    expect(source).toContain('web-terminal__viewport')
    expect(source).toContain("$t('webTerminal.activationTitle')")
    expect(source).toContain("$t('webTerminal.connectTitle')")
    expect(source).toContain("$t('webTerminal.leaveTitle')")
    expect(source).toContain("$t('webTerminal.connect')")
    expect(source).toContain("$t('webTerminal.disconnect')")
  })
})
