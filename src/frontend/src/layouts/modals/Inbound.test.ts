import { beforeEach, describe, expect, it, vi } from 'vitest'

const mockLoadInbounds = vi.hoisted(() => vi.fn())

vi.mock('@/store/modules/data', () => ({
  default: () => ({
    loadInbounds: mockLoadInbounds,
  }),
}))

const passthrough = { render: () => null }

vi.mock('@/components/Dial.vue', () => ({ default: passthrough }))
vi.mock('@/components/Listen.vue', () => ({ default: passthrough }))
vi.mock('@/components/protocols/Direct.vue', () => ({ default: passthrough }))
vi.mock('@/components/Users.vue', () => ({ default: passthrough }))
vi.mock('@/components/protocols/Shadowsocks.vue', () => ({ default: passthrough }))
vi.mock('@/components/protocols/Hysteria.vue', () => ({ default: passthrough }))
vi.mock('@/components/protocols/Hysteria2.vue', () => ({ default: passthrough }))
vi.mock('@/components/protocols/Naive.vue', () => ({ default: passthrough }))
vi.mock('@/components/protocols/ShadowTls.vue', () => ({ default: passthrough }))
vi.mock('@/components/protocols/Tuic.vue', () => ({ default: passthrough }))
vi.mock('@/components/protocols/Tun.vue', () => ({ default: passthrough }))
vi.mock('@/components/protocols/AnyTls.vue', () => ({ default: passthrough }))
vi.mock('@/components/tls/InTLS.vue', () => ({ default: passthrough }))
vi.mock('@/components/protocols/TProxy.vue', () => ({ default: passthrough }))
vi.mock('@/components/Multiplex.vue', () => ({ default: passthrough }))
vi.mock('@/components/Transport.vue', () => ({ default: passthrough }))
vi.mock('@/components/Addr.vue', () => ({ default: passthrough }))
vi.mock('@/components/OutJson.vue', () => ({ default: passthrough }))

const VuetifyPassthrough = {
  props: ['title', 'text'],
  setup(props: { title?: string, text?: string }, { attrs, slots }: any) {
    return () => [props.title, props.text, slots.default?.(), attrs]
  },
}

const vuetifyComponentMocks = {
  VAlert: VuetifyPassthrough,
  VBtn: VuetifyPassthrough,
  VCard: VuetifyPassthrough,
  VCardActions: VuetifyPassthrough,
  VCardText: VuetifyPassthrough,
  VCardTitle: VuetifyPassthrough,
  VCardSubtitle: VuetifyPassthrough,
  VChip: VuetifyPassthrough,
  VCol: VuetifyPassthrough,
  VContainer: VuetifyPassthrough,
  VDialog: VuetifyPassthrough,
  VDivider: VuetifyPassthrough,
  VIcon: VuetifyPassthrough,
  VRow: VuetifyPassthrough,
  VSelect: VuetifyPassthrough,
  VSkeletonLoader: VuetifyPassthrough,
  VSpacer: VuetifyPassthrough,
  VTab: VuetifyPassthrough,
  VTabs: VuetifyPassthrough,
  VTextField: VuetifyPassthrough,
  VWindow: VuetifyPassthrough,
  VWindowItem: VuetifyPassthrough,
}

vi.mock('vuetify/components', () => vuetifyComponentMocks)
vi.mock('vuetify/lib/components/VAlert/index.mjs', () => ({ VAlert: VuetifyPassthrough }))
vi.mock('vuetify/lib/components/VBtn/index.mjs', () => ({ VBtn: VuetifyPassthrough }))
vi.mock('vuetify/lib/components/VCard/index.mjs', () => ({
  VCard: VuetifyPassthrough,
  VCardActions: VuetifyPassthrough,
  VCardText: VuetifyPassthrough,
  VCardTitle: VuetifyPassthrough,
  VCardSubtitle: VuetifyPassthrough,
}))
vi.mock('vuetify/lib/components/VChip/index.mjs', () => ({ VChip: VuetifyPassthrough }))
vi.mock('vuetify/lib/components/VDialog/index.mjs', () => ({ VDialog: VuetifyPassthrough }))
vi.mock('vuetify/lib/components/VDivider/index.mjs', () => ({ VDivider: VuetifyPassthrough }))
vi.mock('vuetify/lib/components/VGrid/index.mjs', () => ({
  VCol: VuetifyPassthrough,
  VContainer: VuetifyPassthrough,
  VRow: VuetifyPassthrough,
}))
vi.mock('vuetify/lib/components/VIcon/index.mjs', () => ({ VIcon: VuetifyPassthrough }))
vi.mock('vuetify/lib/components/VSelect/index.mjs', () => ({ VSelect: VuetifyPassthrough }))
vi.mock('vuetify/lib/components/VSkeletonLoader/index.mjs', () => ({ VSkeletonLoader: VuetifyPassthrough }))
vi.mock('vuetify/lib/components/VSpacer/index.mjs', () => ({ VSpacer: VuetifyPassthrough }))
vi.mock('vuetify/lib/components/VTabs/index.mjs', () => ({ VTab: VuetifyPassthrough, VTabs: VuetifyPassthrough }))
vi.mock('vuetify/lib/components/VTextField/index.mjs', () => ({ VTextField: VuetifyPassthrough }))
vi.mock('vuetify/lib/components/VWindow/index.mjs', () => ({
  VWindow: VuetifyPassthrough,
  VWindowItem: VuetifyPassthrough,
}))

describe('Inbound modal', () => {
  beforeEach(() => {
    mockLoadInbounds.mockReset()
  })

  it('stops loading and records an error when the selected inbound is missing', async () => {
    mockLoadInbounds.mockResolvedValue(null)
    const { default: InboundModal } = await import('./Inbound.vue')
    const component = InboundModal as any
    const vm: any = {
      ...component.data(),
      $props: { readonly: true },
    }

    await expect(component.methods.loadData.call(vm, 13)).resolves.toBeUndefined()

    expect(vm.loading).toBe(false)
    expect(vm.loadError).toBe('Inbound 13 was not found on the selected panel.')
    expect(vm.inbound.id).toBe(13)
    expect(vm.inbound.tag).toBe('missing-inbound-13')
  })
})
