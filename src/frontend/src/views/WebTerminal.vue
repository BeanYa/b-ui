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
          <span class="app-page__hero-meta-item">{{ session.transcript.length }} events</span>
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
            <div v-if="session.transcript.length === 0" class="web-terminal__placeholder">
              Connect to start the terminal session.
            </div>
            <pre v-else>{{ transcriptText }}</pre>
          </div>
        </div>

        <div>
          <div class="web-terminal__label">Command</div>
          <div class="web-terminal__composer">
            <v-text-field
              v-model="command"
              hide-details
              density="comfortable"
              placeholder="Enter a command"
              :disabled="!canSend"
              @keydown.enter.prevent="sendCommand"
            ></v-text-field>
            <v-btn color="primary" :disabled="!canSend || !command.trim()" @click="sendCommand">Send</v-btn>
          </div>
        </div>
      </v-card-text>
    </v-card>
  </div>
</template>

<script lang="ts" setup>
import { computed, onBeforeUnmount, ref, shallowRef } from 'vue'

import {
  createWebTerminalSession,
  reduceWebTerminalSession,
  type WebTerminalServerMessage,
} from '@/features/webterminal/session'

const session = ref(createWebTerminalSession())
const command = ref('')
const socket = shallowRef<WebSocket | null>(null)

const applySession = (action: Parameters<typeof reduceWebTerminalSession>[1]) => {
  session.value = reduceWebTerminalSession(session.value, action)
}

const canConnect = computed(() => session.value.status === 'disconnected')
const canDisconnect = computed(() => session.value.status !== 'disconnected')
const canSend = computed(() => session.value.status === 'connected' && socket.value !== null)
const transcriptText = computed(() => session.value.transcript.map(entry => entry.text).join('\n'))

const connect = () => {
  if (!canConnect.value) return

  const baseUrl = (window as any).BASE_URL ?? '/'
  const protocol = window.location.protocol === 'https:' ? 'wss://' : 'ws://'

  applySession({
    type: 'connection',
    status: 'connecting',
    text: 'Opening terminal connection...',
  })

  const currentSocket = new WebSocket(`${protocol}${window.location.host}${baseUrl}api/webssh/ws`)
  socket.value = currentSocket

  currentSocket.addEventListener('open', () => {
    if (socket.value !== currentSocket) return

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
    } catch {
      applySession({
        type: 'connection',
        status: session.value.status,
        text: 'Ignored malformed terminal message.',
      })
    }
  })

  currentSocket.addEventListener('error', () => {
    if (socket.value !== currentSocket) return

    socket.value = null
    currentSocket.close()

    applySession({
      type: 'connection',
      status: 'disconnected',
      text: 'Terminal connection error.',
    })
  })

  currentSocket.addEventListener('close', () => {
    if (socket.value === currentSocket) {
      socket.value = null

      applySession({
        type: 'connection',
        status: 'disconnected',
        text: 'Terminal disconnected.',
      })
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
}

const sendCommand = () => {
  if (!canSend.value || command.value.trim().length === 0 || socket.value === null) return

  socket.value.send(JSON.stringify({ type: 'input', data: `${command.value}\n` }))
  command.value = ''
}

onBeforeUnmount(() => {
  const currentSocket = socket.value
  if (!currentSocket) return

  socket.value = null
  currentSocket.close()
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
  background: rgb(var(--v-theme-surface-variant));
  border-radius: 16px;
  min-height: 280px;
  padding: 16px;
}

.web-terminal__transcript pre {
  font-family: 'Geist Mono Variable', monospace;
  margin: 0;
  white-space: pre-wrap;
  word-break: break-word;
}

.web-terminal__placeholder {
  color: rgba(var(--v-theme-on-surface), 0.7);
}

.web-terminal__composer {
  display: grid;
  gap: 12px;
  grid-template-columns: minmax(0, 1fr) auto;
}

@media (max-width: 720px) {
  .web-terminal__toolbar,
  .web-terminal__composer {
    grid-template-columns: 1fr;
  }

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
