import { describe, expect, it } from 'vitest'

import {
  createWebTerminalSession,
  reduceWebTerminalSession,
  type WebTerminalServerMessage,
} from './session'

describe('web terminal session reducer', () => {
  it('starts disconnected with an empty transcript', () => {
    expect(createWebTerminalSession()).toEqual({
      status: 'disconnected',
      transcript: [],
    })
  })

  it('tracks connection status changes and records status notes', () => {
    const connecting = reduceWebTerminalSession(createWebTerminalSession(), {
      type: 'connection',
      status: 'connecting',
      text: 'Opening terminal connection...',
    })

    const connected = reduceWebTerminalSession(connecting, {
      type: 'connection',
      status: 'connected',
      text: 'Terminal connected.',
    })

    expect(connected.status).toBe('connected')
    expect(connected.transcript).toEqual([
      { kind: 'status', text: 'Opening terminal connection...' },
      { kind: 'status', text: 'Terminal connected.' },
    ])
  })

  it('appends server output and status messages to the transcript', () => {
    const messages: WebTerminalServerMessage[] = [
      { type: 'output', data: 'pwd\n' },
      { type: 'status', data: 'Command completed' },
    ]

    const session = messages.reduce((state, message) => reduceWebTerminalSession(state, {
      type: 'server-message',
      message,
    }), createWebTerminalSession())

    expect(session.transcript).toEqual([
      { kind: 'output', text: 'pwd\n' },
      { kind: 'status', text: 'Command completed' },
    ])
  })

  it('ignores unsupported server payloads without changing state', () => {
    const session = reduceWebTerminalSession(createWebTerminalSession(), {
      type: 'server-message',
      message: { type: 'unknown', data: 'noop' } as WebTerminalServerMessage,
    })

    expect(session).toEqual(createWebTerminalSession())
  })
})
