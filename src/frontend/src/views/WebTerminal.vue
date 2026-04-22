<template>
  <div class="app-page">
    <section class="app-page__hero">
      <div class="app-page__hero-head">
        <div class="app-page__hero-kicker">WebTerminal</div>
        <h1 class="app-page__hero-title">WebTerminal</h1>
        <p class="app-page__hero-copy">
          Open a browser terminal session to inspect the local terminal-backed backend stream without leaving the admin UI.
        </p>
        <div class="app-page__hero-meta">
          <span class="app-page__hero-meta-item">Connection status: {{ session.status }}</span>
          <span class="app-page__hero-meta-item">Interactive TTY stream</span>
        </div>
      </div>
    </section>

    <v-card class="app-card-shell web-terminal" elevation="5">
      <v-card-text class="web-terminal__stack">
        <div class="web-terminal__toolbar">
          <div>
            <div class="web-terminal__label">Connection status</div>
            <div class="web-terminal__status" :data-status="session.status">{{ session.status }}</div>
          </div>
          <div class="web-terminal__actions">
            <v-btn color="primary" :disabled="!canConnect" @click="connect">Connect</v-btn>
            <v-btn variant="outlined" color="warning" :disabled="!canDisconnect" @click="disconnect">Disconnect</v-btn>
          </div>
        </div>

        <div>
          <div class="web-terminal__label">Transcript</div>
          <div class="web-terminal__transcript" role="log" aria-live="polite">
            <div ref="terminalHost" class="web-terminal__viewport"></div>
            <div v-if="session.status !== 'connected'" class="web-terminal__placeholder">
              Connect to start the interactive terminal session.
            </div>
          </div>
        </div>
      </v-card-text>
    </v-card>
  </div>
</template>

<script lang="ts" setup>
import { computed, onBeforeUnmount, onMounted, ref, shallowRef } from 'vue'
import { FitAddon } from '@xterm/addon-fit'
import { Terminal } from '@xterm/xterm'
import '@xterm/xterm/css/xterm.css'

import {
  createWebTerminalSession,
  reduceWebTerminalSession,
  type WebTerminalServerMessage,
} from '@/features/webterminal/session'

const session = ref(createWebTerminalSession())
const socket = shallowRef<WebSocket | null>(null)
const terminalHost = ref<HTMLDivElement | null>(null)
const terminal = shallowRef<Terminal | null>(null)
const fitAddon = shallowRef<FitAddon | null>(null)
const resizeObserver = shallowRef<ResizeObserver | null>(null)
const terminalInputSubscription = shallowRef<{ dispose: () => void } | null>(null)

const applySession = (action: Parameters<typeof reduceWebTerminalSession>[1]) => {
  session.value = reduceWebTerminalSession(session.value, action)
}

const canConnect = computed(() => session.value.status === 'disconnected')
const canDisconnect = computed(() => session.value.status !== 'disconnected')

const writeTerminalStatusLine = (text: string) => {
  terminal.value?.writeln(`\r\n${text}`)
}

const sendResize = () => {
  const currentTerminal = terminal.value
  const currentSocket = socket.value
  if (!currentTerminal || !currentSocket || currentSocket.readyState !== WebSocket.OPEN) return

  currentSocket.send(JSON.stringify({
    type: 'resize',
    cols: currentTerminal.cols,
    rows: currentTerminal.rows,
  }))
}

const connect = () => {
  if (!canConnect.value) return

  const rawBaseUrl = String((window as any).BASE_URL ?? '/')
  const normalizedBaseUrl = rawBaseUrl.endsWith('/') ? rawBaseUrl : `${rawBaseUrl}/`
  const wsUrl = new URL(`${normalizedBaseUrl}api/webssh/ws`, window.location.origin)
  wsUrl.protocol = window.location.protocol === 'https:' ? 'wss:' : 'ws:'

  applySession({
    type: 'connection',
    status: 'connecting',
    text: 'Opening terminal connection...',
  })

  terminal.value?.reset()
  terminal.value?.focus()
  writeTerminalStatusLine('Opening terminal connection...')

  console.debug('[WebTerminal] connecting websocket', {
    rawBaseUrl,
    normalizedBaseUrl,
    url: wsUrl.toString(),
  })

  const currentSocket = new WebSocket(wsUrl)
  socket.value = currentSocket

  currentSocket.addEventListener('open', () => {
    if (socket.value !== currentSocket) return

    terminal.value?.focus()
    fitAddon.value?.fit()
    sendResize()

    applySession({
      type: 'connection',
      status: 'connected',
      text: 'Terminal connected.',
    })
  })

  currentSocket.addEventListener('message', (event) => {
    if (socket.value !== currentSocket) return

    try {
      const message = JSON.parse(String(event.data)) as WebTerminalServerMessage

      applySession({
        type: 'server-message',
        message,
      })

      if (message.type === 'output') {
        terminal.value?.write(message.data)
      }

      if (message.type === 'status') {
        writeTerminalStatusLine(message.data)
      }
    } catch {
      console.warn('[WebTerminal] ignored malformed websocket payload', {
        payload: event.data,
      })

      applySession({
        type: 'connection',
        status: session.value.status,
        text: 'Ignored malformed terminal message.',
      })
    }
  })

  currentSocket.addEventListener('error', () => {
    if (socket.value !== currentSocket) return

    console.error('[WebTerminal] websocket error event')

    socket.value = null
    currentSocket.close()

    applySession({
      type: 'connection',
      status: 'disconnected',
      text: 'Terminal connection error.',
    })

    writeTerminalStatusLine('Terminal connection error.')
  })

  currentSocket.addEventListener('close', () => {
    console.debug('[WebTerminal] websocket closed')

    if (socket.value === currentSocket) {
      socket.value = null

      applySession({
        type: 'connection',
        status: 'disconnected',
        text: 'Terminal disconnected.',
      })

      writeTerminalStatusLine('Terminal disconnected.')
    }
  })
}

const disconnect = () => {
  const currentSocket = socket.value
  if (!currentSocket) return

  socket.value = null
  currentSocket.close()

  applySession({
    type: 'connection',
    status: 'disconnected',
    text: 'Terminal disconnected.',
  })

  writeTerminalStatusLine('Terminal disconnected.')
}

onMounted(() => {
  const host = terminalHost.value
  if (!host) return

  const currentTerminal = new Terminal({
    convertEol: false,
    cursorBlink: true,
    fontFamily: 'Geist Mono Variable, monospace',
    fontSize: 15,
    lineHeight: 1.35,
    theme: {
      background: '#000000',
      foreground: '#e5e7eb',
      cursor: '#e5e7eb',
      selectionBackground: '#374151',
    },
  })
  const currentFitAddon = new FitAddon()

  currentTerminal.loadAddon(currentFitAddon)
  currentTerminal.open(host)
  currentFitAddon.fit()
  currentTerminal.focus()
  currentTerminal.writeln('Press Connect to start the interactive terminal session.')

  terminalInputSubscription.value = currentTerminal.onData((data) => {
    if (session.value.status !== 'connected' || socket.value === null || socket.value.readyState !== WebSocket.OPEN) return

    socket.value.send(JSON.stringify({ type: 'input', data }))
  })

  terminal.value = currentTerminal
  fitAddon.value = currentFitAddon

  const observer = new ResizeObserver(() => {
    currentFitAddon.fit()
    sendResize()
  })
  observer.observe(host)
  resizeObserver.value = observer
})

onBeforeUnmount(() => {
  const currentSocket = socket.value
  if (currentSocket) {
    socket.value = null
    currentSocket.close()
  }

  terminalInputSubscription.value?.dispose()
  terminalInputSubscription.value = null
  resizeObserver.value?.disconnect()
  resizeObserver.value = null
  terminal.value?.dispose()
  terminal.value = null
})
</script>

<style scoped>
.web-terminal__stack {
  display: grid;
  gap: 20px;
}

.web-terminal__toolbar {
  align-items: center;
  display: flex;
  gap: 16px;
  justify-content: space-between;
}

.web-terminal__actions {
  display: flex;
  gap: 12px;
}

.web-terminal__label {
  font-size: 13px;
  font-weight: 700;
  letter-spacing: 0.08em;
  margin-bottom: 8px;
  text-transform: uppercase;
}

.web-terminal__status {
  font-family: 'Geist Mono Variable', monospace;
}

.web-terminal__transcript {
  background: #000;
  border-radius: 16px;
  display: grid;
  min-height: 280px;
  position: relative;
  padding: 16px;
}

.web-terminal__viewport {
  min-height: 320px;
  width: 100%;
}

.web-terminal__viewport :deep(.xterm),
.web-terminal__viewport :deep(.xterm-viewport),
.web-terminal__viewport :deep(.xterm-screen) {
  border-radius: 12px;
}

.web-terminal__placeholder {
  align-self: start;
  color: rgba(229, 231, 235, 0.72);
  left: 22px;
  pointer-events: none;
  position: absolute;
  top: 22px;
}

@media (max-width: 720px) {
  .web-terminal__toolbar {
    align-items: stretch;
    flex-direction: column;
  }

  .web-terminal__actions {
    width: 100%;
  }

  .web-terminal__actions :deep(.v-btn) {
    flex: 1;
  }
}
</style>
