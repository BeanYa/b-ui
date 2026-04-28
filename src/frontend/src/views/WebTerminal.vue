<template>
  <div class="app-page">
    <section class="app-page__hero">
      <div class="app-page__hero-head">
        <div class="app-page__hero-kicker">{{ $t('pages.webTerminal') }}</div>
        <h1 class="app-page__hero-title">{{ $t('pages.webTerminal') }}</h1>
        <p class="app-page__hero-copy">
          {{ $t('webTerminal.heroCopy') }}
        </p>
        <div class="app-page__hero-meta">
          <span class="app-page__hero-meta-item">{{ $t('webTerminal.connectionStatus') }}: {{ session.status }}</span>
          <span class="app-page__hero-meta-item">{{ $t('webTerminal.interactiveStream') }}</span>
        </div>
      </div>
    </section>

    <v-card class="app-card-shell web-terminal" elevation="5">
      <v-card-text class="web-terminal__stack">
        <div class="web-terminal__toolbar">
          <div>
            <div class="web-terminal__label">{{ $t('webTerminal.connectionStatus') }}</div>
            <div class="web-terminal__status" :data-status="session.status">{{ session.status }}</div>
          </div>
          <div class="web-terminal__actions">
            <v-btn color="primary" :disabled="!canConnect" @click="requestConnect">{{ $t('webTerminal.connect') }}</v-btn>
            <v-btn variant="outlined" color="warning" :disabled="!canDisconnect" @click="disconnect">{{ $t('webTerminal.disconnect') }}</v-btn>
          </div>
        </div>

        <div>
          <div class="web-terminal__label">{{ $t('webTerminal.transcript') }}</div>
          <div class="web-terminal__transcript" role="log" aria-live="polite">
            <div ref="terminalHost" class="web-terminal__viewport"></div>
            <div v-if="showActivationOverlay" class="web-terminal__placeholder">
              {{ $t('webTerminal.placeholder') }}
            </div>
          </div>
        </div>
      </v-card-text>

      <div v-if="showActivationOverlay" class="web-terminal__activation-mask">
        <div class="web-terminal__activation-dialog">
          <div class="web-terminal__activation-title">{{ $t('webTerminal.activationTitle') }}</div>
          <p class="web-terminal__activation-copy">
            {{ $t('webTerminal.activationCopy') }}
          </p>
          <v-btn color="primary" size="large" @click="requestConnect">{{ $t('webTerminal.connect') }}</v-btn>
        </div>
      </div>
    </v-card>

    <v-dialog v-model="connectDialogVisible" class="app-dialog app-dialog--compact" max-width="440">
      <v-card class="app-card-shell">
        <v-card-title>{{ $t('webTerminal.connectTitle') }}</v-card-title>
        <v-card-text>
          {{ $t('webTerminal.connectCopy') }}
        </v-card-text>
        <v-card-actions class="web-terminal__dialog-actions">
          <v-spacer></v-spacer>
          <v-btn variant="text" @click="connectDialogVisible = false">{{ $t('no') }}</v-btn>
          <v-btn color="primary" @click="confirmConnect">{{ $t('webTerminal.connect') }}</v-btn>
        </v-card-actions>
      </v-card>
    </v-dialog>

    <v-dialog v-model="leaveDialogVisible" class="app-dialog app-dialog--compact" max-width="460">
      <v-card class="app-card-shell">
        <v-card-title>{{ $t('webTerminal.leaveTitle') }}</v-card-title>
        <v-card-text>
          {{ $t('webTerminal.leaveCopy') }}
        </v-card-text>
        <v-card-actions class="web-terminal__dialog-actions">
          <v-spacer></v-spacer>
          <v-btn variant="text" @click="cancelLeave">{{ $t('webTerminal.stay') }}</v-btn>
          <v-btn color="warning" @click="confirmLeave">{{ $t('webTerminal.leaveAndAbort') }}</v-btn>
        </v-card-actions>
      </v-card>
    </v-dialog>
  </div>
</template>

<script lang="ts" setup>
import { computed, onBeforeUnmount, onMounted, ref, shallowRef } from 'vue'
import { FitAddon } from '@xterm/addon-fit'
import { Terminal } from '@xterm/xterm'
import { onBeforeRouteLeave } from 'vue-router'
import '@xterm/xterm/css/xterm.css'

import { i18n } from '@/locales'
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
const connectDialogVisible = ref(false)
const leaveDialogVisible = ref(false)
const pendingLeaveTarget = ref<string | null>(null)
const pendingLeaveDecision = shallowRef<{
  proceed: () => void
  cancel: () => void
} | null>(null)

const applySession = (action: Parameters<typeof reduceWebTerminalSession>[1]) => {
  session.value = reduceWebTerminalSession(session.value, action)
}

const canConnect = computed(() => session.value.status === 'disconnected')
const canDisconnect = computed(() => session.value.status !== 'disconnected')
const hasActiveSession = computed(() => session.value.status !== 'disconnected')
const showActivationOverlay = computed(() => session.value.status === 'disconnected')
const terminalText = (key: string) => i18n.global.t(`webTerminal.${key}`).toString()

const writeTerminalStatusLine = (text: string) => {
  terminal.value?.writeln(`\r\n${text}`)
}

const closeActiveSession = (reasonText = terminalText('terminalDisconnected')) => {
  const currentSocket = socket.value
  if (currentSocket) {
    socket.value = null
    currentSocket.close()
  }

  if (session.value.status !== 'disconnected') {
    applySession({
      type: 'connection',
      status: 'disconnected',
      text: reasonText,
    })
  }

  writeTerminalStatusLine(reasonText)
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

const requestConnect = () => {
  if (!canConnect.value) return

  console.debug('[WebTerminal] connect confirmation opened')
  connectDialogVisible.value = true
}

const confirmConnect = () => {
  connectDialogVisible.value = false
  connect()
}

const connect = () => {
  if (!canConnect.value) return

  const rawBaseUrl = String((window as any).BASE_URL ?? '/')
  const normalizedBaseUrl = rawBaseUrl.endsWith('/') ? rawBaseUrl : `${rawBaseUrl}/`
  const wsUrl = new URL(`${normalizedBaseUrl}api/webssh/ws`, window.location.origin)
  wsUrl.protocol = window.location.protocol === 'https:' ? 'wss:' : 'ws:'
  const openingText = terminalText('terminalOpening')

  applySession({
    type: 'connection',
    status: 'connecting',
    text: openingText,
  })

  terminal.value?.reset()
  terminal.value?.focus()
  writeTerminalStatusLine(openingText)

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
      text: terminalText('terminalConnected'),
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
        text: terminalText('terminalMalformedMessage'),
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
      text: terminalText('terminalConnectionError'),
    })

    writeTerminalStatusLine(terminalText('terminalConnectionError'))
  })

  currentSocket.addEventListener('close', () => {
    console.debug('[WebTerminal] websocket closed')

    if (socket.value === currentSocket) {
      socket.value = null

      applySession({
        type: 'connection',
        status: 'disconnected',
        text: terminalText('terminalDisconnected'),
      })

      writeTerminalStatusLine(terminalText('terminalDisconnected'))
    }
  })
}

const disconnect = () => {
  closeActiveSession()
}

const cancelLeave = () => {
  console.debug('[WebTerminal] canceled blocked route leave', {
    targetPath: pendingLeaveTarget.value,
  })

  pendingLeaveDecision.value?.cancel()
  pendingLeaveDecision.value = null
  pendingLeaveTarget.value = null
  leaveDialogVisible.value = false
}

const confirmLeave = () => {
  const targetPath = pendingLeaveTarget.value
  const decision = pendingLeaveDecision.value

  leaveDialogVisible.value = false
  pendingLeaveDecision.value = null
  pendingLeaveTarget.value = null

  console.debug('[WebTerminal] resuming blocked route leave and aborting active session', {
    targetPath,
  })

  closeActiveSession(terminalText('terminalAborted'))
  decision?.proceed()
}

const handleBeforeUnload = (event: BeforeUnloadEvent) => {
  if (!hasActiveSession.value) return

  event.preventDefault()
  event.returnValue = terminalText('beforeUnload')
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
  currentTerminal.writeln(terminalText('terminalStartPrompt'))

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

  window.addEventListener('beforeunload', handleBeforeUnload)
})

onBeforeRouteLeave((to, _from, next) => {
  if (!hasActiveSession.value) {
    next()
    return
  }

  console.debug('[WebTerminal] route leave confirmation opened', {
    to: to.fullPath,
    status: session.value.status,
  })

  pendingLeaveDecision.value?.cancel()
  pendingLeaveTarget.value = to.fullPath
  pendingLeaveDecision.value = {
    proceed: () => next(),
    cancel: () => next(false),
  }
  leaveDialogVisible.value = true
})

onBeforeUnmount(() => {
  window.removeEventListener('beforeunload', handleBeforeUnload)

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

.web-terminal {
  overflow: hidden;
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

.web-terminal__activation-mask {
  align-items: center;
  backdrop-filter: blur(4px);
  background: linear-gradient(180deg, rgba(8, 10, 14, 0.58), rgba(8, 10, 14, 0.72));
  display: flex;
  inset: 0;
  justify-content: center;
  padding: 24px;
  position: absolute;
  z-index: 2;
}

.web-terminal__activation-dialog {
  align-items: center;
  background: color-mix(in srgb, var(--app-surface-2) 92%, transparent);
  border: 1px solid var(--app-border-1);
  border-radius: 20px;
  box-shadow: var(--app-shadow-ring), var(--app-shadow-panel);
  display: grid;
  gap: 14px;
  justify-items: center;
  max-width: 420px;
  padding: 24px;
  text-align: center;
}

.web-terminal__activation-title {
  color: var(--app-text-1);
  font-size: 20px;
  font-weight: 700;
}

.web-terminal__activation-copy {
  color: var(--app-text-2);
  line-height: 1.6;
  margin: 0;
}

.web-terminal__dialog-actions {
  padding: 0 16px 16px;
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
