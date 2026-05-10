import { fileURLToPath } from 'node:url'
import { readFileSync } from 'node:fs'
import { describe, expect, it } from 'vitest'

const source = (path: string) => readFileSync(fileURLToPath(new URL(path, import.meta.url)), 'utf8')

describe('domain-managed local panel resources', () => {
  it('uses view actions for Hub-managed inbound, user, and TLS records', () => {
    const inbounds = source('./Inbounds.vue')
    const clients = source('./Clients.vue')
    const tls = source('./Tls.vue')

    expect(inbounds).toContain("'mdi-eye'")
    expect(inbounds).toContain("showModal(item.id, isClusterManaged(item))")
    expect(inbounds).toContain(":readonly=\"modal.readonly\"")

    expect(clients).toContain('mdi-eye')
    expect(clients).toContain('showModal(item.id, true)')
    expect(clients).toContain(':readonly="modal.readonly"')

    expect(tls).toContain("'mdi-eye'")
    expect(tls).toContain("showModal(item.id, undefined, isClusterManaged(item))")
    expect(tls).toContain(":readonly=\"modal.readonly\"")
  })

  it('keeps read-only modals inspectable while hiding save and generation actions', () => {
    const inboundModal = source('../layouts/modals/Inbound.vue')
    const clientModal = source('../layouts/modals/Client.vue')
    const tlsModal = source('../layouts/modals/Tls.vue')

    expect(inboundModal).toContain("props: ['visible', 'id', 'inTags', 'tlsConfigs', 'readonly']")
    expect(inboundModal).toContain(':disabled="readonly"')
    expect(inboundModal).toContain('v-if="!readonly"')
    expect(inboundModal).toContain('this.$props.readonly')

    expect(clientModal).toContain("props: ['visible', 'id', 'inboundTags', 'groups', 'readonly']")
    expect(clientModal).toContain(':disabled="readonly"')
    expect(clientModal).toContain('v-if="!readonly"')
    expect(clientModal).toContain('this.$props.readonly')

    expect(tlsModal).toContain("props: ['visible', 'data', 'id', 'readonly']")
    expect(tlsModal).toContain(':disabled="readonly"')
    expect(tlsModal).toContain('v-if="!readonly"')
    expect(tlsModal).toContain('this.$props.readonly')
  })
})
