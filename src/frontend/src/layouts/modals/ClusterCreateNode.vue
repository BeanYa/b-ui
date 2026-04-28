<template>
  <v-dialog class="app-dialog app-dialog--wide" transition="dialog-bottom-transition" width="900" v-model="dialogVisible">
    <v-card class="rounded-lg" :loading="loading">
      <v-card-title>
        {{ $t('actions.add') + " " + $t('objects.inbound') }}
      </v-card-title>
      <v-divider></v-divider>

      <v-card-text class="app-dialog__body">
        <v-container style="padding: 0;">

          <!-- Section 1: TLS Settings (optional, collapsible) -->
          <v-card class="rounded-lg mb-4">
            <v-card-title
              class="cursor-pointer d-flex align-center"
              @click="tlsExpanded = !tlsExpanded"
            >
              {{ $t('objects.tls') }}
              <v-icon class="ml-2">{{ tlsExpanded ? 'mdi-chevron-up' : 'mdi-chevron-down' }}</v-icon>
            </v-card-title>
            <v-divider v-if="tlsExpanded"></v-divider>
            <v-card-text v-if="tlsExpanded">
              <v-row>
                <v-col cols="12" sm="6" md="4">
                  <v-select
                    hide-details
                    :label="$t('objects.tls')"
                    :items="tlsConfigItems"
                    clearable
                    v-model="selectedTlsId"
                    @update:modelValue="onTlsConfigSelected"
                  ></v-select>
                </v-col>
              </v-row>
              <template v-if="!selectedTlsId">
                <v-row class="mt-2">
                  <v-col cols="12" sm="6" md="4">
                    <v-text-field
                      :label="$t('client.name')"
                      hide-details
                      v-model="tlsName"
                    ></v-text-field>
                  </v-col>
                  <v-col cols="12" sm="6" md="4">
                    <v-menu location="bottom">
                      <template v-slot:activator="{ props }">
                        <v-btn v-bind="props" block variant="tonal">{{ $t('tls.applyPreset') }}</v-btn>
                      </template>
                      <v-list>
                        <v-list-item
                          v-for="preset in presetItems"
                          :key="preset.value"
                          :title="preset.title"
                          @click="applyTlsPreset(preset.value)"
                        />
                      </v-list>
                    </v-menu>
                  </v-col>
                </v-row>
              </template>
            </v-card-text>
          </v-card>

          <!-- Section 2: Inbound Configuration (required) -->
          <v-card class="rounded-lg mb-4">
            <v-card-title>{{ $t('objects.inbound') }}</v-card-title>
            <v-divider></v-divider>
            <v-card-text>
              <v-row>
                <v-col cols="12" sm="6" md="4">
                  <v-select
                    hide-details
                    :label="$t('type')"
                    :items="Object.keys(inTypes).map((key, index) => ({ title: key, value: Object.values(inTypes)[index] }))"
                    v-model="inbound.type"
                    @update:modelValue="changeType"
                  ></v-select>
                </v-col>
                <v-col cols="12" sm="6" md="4">
                  <v-text-field
                    v-model="inbound.tag"
                    :label="$t('objects.tag')"
                    hide-details
                  ></v-text-field>
                </v-col>
              </v-row>
              <v-row>
                <v-col cols="12" sm="6" md="4">
                  <v-text-field
                    v-model="inbound.listen"
                    :label="$t('objects.listen')"
                    hide-details
                  ></v-text-field>
                </v-col>
                <v-col cols="12" sm="6" md="4">
                  <v-text-field
                    v-model.number="inbound.listen_port"
                    :label="$t('in.port')"
                    type="number"
                    min="1"
                    max="65535"
                    hide-details
                  ></v-text-field>
                </v-col>
              </v-row>
            </v-card-text>
          </v-card>

          <!-- Section 3: User Management (optional, collapsible) -->
          <v-card class="rounded-lg mb-4">
            <v-card-title
              class="cursor-pointer d-flex align-center"
              @click="usersExpanded = !usersExpanded"
            >
              {{ $t('objects.user') }}
              <v-icon class="ml-2">{{ usersExpanded ? 'mdi-chevron-up' : 'mdi-chevron-down' }}</v-icon>
            </v-card-title>
            <v-divider v-if="usersExpanded"></v-divider>
            <v-card-text v-if="usersExpanded">
              <v-row>
                <v-col cols="12">
                  <v-text-field
                    v-model="userUsername"
                    :label="$t('client.name')"
                    hide-details
                  ></v-text-field>
                </v-col>
                <v-col cols="12">
                  <v-text-field
                    v-model="userPassword"
                    :label="$t('login.password')"
                    hide-details
                  ></v-text-field>
                </v-col>
              </v-row>
            </v-card-text>
          </v-card>

        </v-container>
      </v-card-text>

      <v-card-actions>
        <v-spacer></v-spacer>
        <v-btn
          color="primary"
          variant="outlined"
          @click="closeModal"
        >
          {{ $t('actions.close') }}
        </v-btn>
        <v-btn
          color="primary"
          variant="tonal"
          :loading="loading"
          :disabled="!validate"
          @click="saveChanges"
        >
          {{ $t('actions.save') }}
        </v-btn>
      </v-card-actions>
    </v-card>
  </v-dialog>
</template>

<script lang="ts">
import { InTypes, createInbound } from '@/types/inbounds'
import type { ProxyCreatePayload } from '@/types/clusterActions'
import { sendAction } from '@/features/clusterPeerApi'
import { useRemoteNodeStore } from '@/store/modules/remoteNode'
import RandomUtil from '@/plugins/randomUtil'
import { push } from 'notivue'
import { i18n } from '@/locales'

export default {
  props: {
    visible: {
      type: Boolean,
      default: false,
    },
  },
  emits: ['close', 'created'],
  data() {
    return {
      loading: false,
      inTypes: InTypes,
      inbound: createInbound('direct', { id: 0, tag: '', listen: '::', listen_port: 0 }),
      tlsExpanded: false,
      selectedTlsId: null as number | null,
      tlsName: '',
      tlsPreset: '',
      usersExpanded: false,
      userUsername: '',
      userPassword: '',
      presetItems: [
        { title: 'Standard', value: 'standard' },
        { title: 'Hysteria2', value: 'hysteria2' },
        { title: 'Reality', value: 'reality' },
      ],
    }
  },
  computed: {
    dialogVisible: {
      get(): boolean {
        return this.$props.visible
      },
      set(v: boolean) {
        if (!v) this.closeModal()
      },
    },
    remoteNode() {
      return useRemoteNodeStore()
    },
    tlsConfigItems() {
      return (this.remoteNode.tlsConfigs?.items ?? []).map((c: any) => ({
        title: c.name ?? `TLS #${c.id}`,
        value: c.id,
      }))
    },
    validate(): boolean {
      if (!this.inbound.tag) return false
      if (this.inbound.listen_port > 65535 || this.inbound.listen_port < 1) return false
      return true
    },
  },
  watch: {
    visible(newValue) {
      if (newValue) {
        this.resetForm()
      }
    },
  },
  methods: {
    resetForm() {
      const port = RandomUtil.randomIntRange(10000, 60000)
      this.inbound = createInbound('direct', { id: 0, tag: 'direct-' + port, listen: '::', listen_port: port })
      this.tlsExpanded = false
      this.selectedTlsId = null
      this.tlsName = ''
      this.tlsPreset = ''
      this.usersExpanded = false
      this.userUsername = ''
      this.userPassword = ''
      this.loading = false
    },
    changeType() {
      if (!this.inbound.listen_port) {
        this.inbound.listen_port = RandomUtil.randomIntRange(10000, 60000)
      }
      const tag = this.inbound.type + '-' + this.inbound.listen_port
      const prevConfig = {
        id: this.inbound.id,
        tag: tag,
        listen: this.inbound.listen ?? '::',
        listen_port: this.inbound.listen_port,
      }
      this.inbound = createInbound(this.inbound.type, prevConfig)
    },
    onTlsConfigSelected(id: number | null) {
      // When an existing config is selected, clear inline fields
      if (id != null) {
        this.tlsName = ''
        this.tlsPreset = ''
      }
    },
    applyTlsPreset(preset: string) {
      this.tlsPreset = preset
      if (!this.tlsName) {
        this.tlsName = preset.charAt(0).toUpperCase() + preset.slice(1)
      }
    },
    closeModal() {
      this.resetForm()
      this.$emit('close')
    },
    async saveChanges() {
      if (!this.validate) return
      this.loading = true

      try {
        const remoteNode = useRemoteNodeStore()

        // Build TLS section
        let tlsPayload: Record<string, unknown> | undefined
        if (this.selectedTlsId != null) {
          tlsPayload = { tls_id: this.selectedTlsId }
        } else if (this.tlsName) {
          tlsPayload = {
            name: this.tlsName,
            preset: this.tlsPreset || undefined,
          }
        }

        // Build users section
        let usersPayload: Record<string, unknown>[] | undefined
        if (this.usersExpanded && (this.userUsername || this.userPassword)) {
          usersPayload = [{
            username: this.userUsername,
            password: this.userPassword,
          }]
        }

        // Strip internal fields before sending
        const { id, addrs, out_json, tls_id, ...inboundData } = this.inbound as any
        if (this.selectedTlsId != null) {
          inboundData.tls_id = this.selectedTlsId
        }

        const payload: ProxyCreatePayload = {
          request_id: RandomUtil.randomUUID(),
          tls: tlsPayload,
          inbound: inboundData,
          users: usersPayload,
        }

        const req = {
          schema_version: 1,
          sourceNodeId: '',
          domain: '',
          sentAt: Math.floor(Date.now() / 1000),
          signature: '',
          action: 'proxy.create',
          payload: payload as unknown as Record<string, unknown>,
        }

        const resp = await sendAction(remoteNode.nodeID, req)

        if (resp.status === 'success') {
          push.success({
            title: i18n.global.t('actions.save').toString(),
            message: 'Proxy created successfully',
          })
          this.$emit('created', resp.data)
          this.closeModal()
        } else {
          push.error({
            title: i18n.global.t('failed').toString(),
            message: resp.error_message ?? 'Unknown error',
          })
        }
      } catch (error: any) {
        push.error({
          title: i18n.global.t('failed').toString(),
          message: error?.message ?? String(error),
        })
      } finally {
        this.loading = false
      }
    },
  },
}
</script>

<style scoped>
.cursor-pointer {
  cursor: pointer;
  user-select: none;
}
</style>
