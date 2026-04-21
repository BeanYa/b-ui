import { readFileSync } from 'node:fs'
import { fileURLToPath } from 'node:url'
import { describe, expect, it } from 'vitest'

describe('WebTerminal view source', () => {
  it('normalizes BASE_URL and opens the terminal websocket with an explicit ws protocol', () => {
    const source = readFileSync(fileURLToPath(new URL('./WebTerminal.vue', import.meta.url)), 'utf8')

    expect(source).toContain("const normalizedBaseUrl = rawBaseUrl.endsWith('/') ? rawBaseUrl : `${rawBaseUrl}/`")
    expect(source).toContain("const wsUrl = new URL(`${normalizedBaseUrl}api/webssh/ws`, window.location.origin)")
    expect(source).toContain("wsUrl.protocol = window.location.protocol === 'https:' ? 'wss:' : 'ws:'")
    expect(source).toContain('const currentSocket = new WebSocket(wsUrl)')
  })

  it('sends newline-terminated input messages for terminal execution', () => {
    const source = readFileSync(fileURLToPath(new URL('./WebTerminal.vue', import.meta.url)), 'utf8')

    expect(source).toMatch(/JSON\.stringify\(\{ type: 'input', data: `\$\{command\.value\}\\n` \}\)/)
  })

  it('guards websocket frame parsing and tears down stale sockets safely on error and close', () => {
    const source = readFileSync(fileURLToPath(new URL('./WebTerminal.vue', import.meta.url)), 'utf8')

    expect(source).toContain('try {')
    expect(source).toContain('JSON.parse(String(event.data)) as WebTerminalServerMessage')
    expect(source).toContain("text: 'Ignored malformed terminal message.'")
    expect(source).toContain("console.warn('[WebTerminal] ignored malformed websocket payload'")
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
