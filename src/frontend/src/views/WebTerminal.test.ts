import { readFileSync } from 'node:fs'
import { fileURLToPath } from 'node:url'
import { describe, expect, it } from 'vitest'

describe('WebTerminal view source', () => {
  it('opens the terminal websocket with an explicit ws protocol and sends newline-terminated input messages', () => {
    const source = readFileSync(fileURLToPath(new URL('./WebTerminal.vue', import.meta.url)), 'utf8')

    expect(source).toContain("const protocol = window.location.protocol === 'https:' ? 'wss://' : 'ws://'")
    expect(source).toContain('new WebSocket(`${protocol}${window.location.host}${baseUrl}api/webssh/ws`)')
    expect(source).toMatch(/JSON\.stringify\(\{ type: 'input', data: `\$\{command\.value\}\\n` \}\)/)
  })

  it('guards websocket frame parsing and tears down stale sockets safely on error and close', () => {
    const source = readFileSync(fileURLToPath(new URL('./WebTerminal.vue', import.meta.url)), 'utf8')

    expect(source).toContain('try {')
    expect(source).toContain('JSON.parse(String(event.data)) as WebTerminalServerMessage')
    expect(source).toContain("text: 'Ignored malformed terminal message.'")
    expect(source).toContain('const currentSocket = new WebSocket')
    expect(source).toContain('if (socket.value === currentSocket) {')
    expect(source).toContain('currentSocket.close()')
  })

  it('renders the terminal status, transcript, command input, and connect controls', () => {
    const source = readFileSync(fileURLToPath(new URL('./WebTerminal.vue', import.meta.url)), 'utf8')

    expect(source).toContain('Connection status')
    expect(source).toContain('Transcript')
    expect(source).toContain('Command')
    expect(source).toContain('Connect')
    expect(source).toContain('Disconnect')
  })
})
