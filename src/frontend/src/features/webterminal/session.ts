export type WebTerminalConnectionStatus = 'disconnected' | 'connecting' | 'connected'

export type WebTerminalTranscriptEntry = {
  kind: 'output' | 'status'
  text: string
}

export type WebTerminalSession = {
  status: WebTerminalConnectionStatus
  transcript: WebTerminalTranscriptEntry[]
}

export type WebTerminalServerMessage = {
  type: 'output' | 'status' | string
  data: string
}

type WebTerminalAction =
  | {
    type: 'connection'
    status: WebTerminalConnectionStatus
    text?: string
  }
  | {
    type: 'server-message'
    message: WebTerminalServerMessage
  }

export const createWebTerminalSession = (): WebTerminalSession => ({
  status: 'disconnected',
  transcript: [],
})

export const reduceWebTerminalSession = (
  state: WebTerminalSession,
  action: WebTerminalAction,
): WebTerminalSession => {
  if (action.type === 'connection') {
    return {
      status: action.status,
      transcript: action.text
        ? [...state.transcript, { kind: 'status', text: action.text }]
        : state.transcript,
    }
  }

  if (action.message.type === 'output') {
    return state
  }

  if (action.message.type !== 'status') {
    return state
  }

  return {
    status: state.status,
    transcript: [...state.transcript, {
      kind: action.message.type,
      text: action.message.data,
    }],
  }
}
