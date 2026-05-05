import { createSSRApp, h } from 'vue'
import { renderToString } from 'vue/server-renderer'
import { describe, expect, it, vi } from 'vitest'

const dataState = {
  inbounds: [
    {
      id: 1,
      tag: 'hub-domain',
      type: 'direct',
      listen: '0.0.0.0',
      listen_port: 443,
      tls_id: 1,
      users: ['alice'],
      cluster_managed: true,
    },
    {
      id: 2,
      tag: 'panel-local',
      type: 'direct',
      listen: '127.0.0.1',
      listen_port: 8443,
      tls_id: 0,
      users: [],
    },
  ],
  tlsConfigs: [],
  endpoints: [],
  onlines: {
    inbound: [],
  },
  enableTraffic: false,
  save: vi.fn(),
  loadInbounds: vi.fn(),
}

vi.mock('@/store/modules/data', () => ({
  default: () => dataState,
}))

vi.mock('@/layouts/modals/Inbound.vue', () => ({
  default: { render: () => null },
}))

vi.mock('@/layouts/modals/Stats.vue', () => ({
  default: { render: () => null },
}))

const VuetifyPassthrough = {
  props: ['title', 'text'],
  setup(props: { title?: string, text?: string }, { attrs, slots }: any) {
    return () => h('div', attrs, [props.title, props.text, slots.default?.()])
  },
}

const vuetifyComponentMocks = {
  VBtn: VuetifyPassthrough,
  VCard: VuetifyPassthrough,
  VCardActions: VuetifyPassthrough,
  VCardText: VuetifyPassthrough,
  VCardTitle: VuetifyPassthrough,
  VChip: VuetifyPassthrough,
  VCol: VuetifyPassthrough,
  VDivider: VuetifyPassthrough,
  VIcon: VuetifyPassthrough,
  VOverlay: VuetifyPassthrough,
  VRow: VuetifyPassthrough,
  VTooltip: VuetifyPassthrough,
}

vi.mock('vuetify/components', () => vuetifyComponentMocks)
vi.mock('vuetify/lib/components/VBtn/index.mjs', () => ({ VBtn: VuetifyPassthrough }))
vi.mock('vuetify/lib/components/VCard/index.mjs', () => ({
  VCard: VuetifyPassthrough,
  VCardActions: VuetifyPassthrough,
  VCardText: VuetifyPassthrough,
  VCardTitle: VuetifyPassthrough,
}))
vi.mock('vuetify/lib/components/VChip/index.mjs', () => ({ VChip: VuetifyPassthrough }))
vi.mock('vuetify/lib/components/VGrid/index.mjs', () => ({ VCol: VuetifyPassthrough, VRow: VuetifyPassthrough }))
vi.mock('vuetify/lib/components/VDivider/index.mjs', () => ({ VDivider: VuetifyPassthrough }))
vi.mock('vuetify/lib/components/VIcon/index.mjs', () => ({ VIcon: VuetifyPassthrough }))
vi.mock('vuetify/lib/components/VOverlay/index.mjs', () => ({ VOverlay: VuetifyPassthrough }))
vi.mock('vuetify/lib/components/VTooltip/index.mjs', () => ({ VTooltip: VuetifyPassthrough }))

const renderInbounds = async () => {
  const { default: Inbounds } = await import('./Inbounds.vue')
  const app = createSSRApp(Inbounds)

  app.config.globalProperties.$t = (key: string) => key
  ;[
    'v-row',
    'v-col',
    'v-card',
    'v-card-title',
    'v-card-text',
    'v-divider',
    'v-card-actions',
    'v-btn',
    'v-icon',
    'v-chip',
    'v-tooltip',
    'v-overlay',
  ].forEach(name => app.component(name, VuetifyPassthrough))

  return renderToString(app)
}

describe('Inbounds view domain-managed cards', () => {
  it('applies domain treatment only to Hub-managed inbounds', async () => {
    const html = await renderInbounds()

    const cardClass = 'app-entity-card inbound-card'
    const domainStart = html.indexOf(cardClass)
    const localStart = html.indexOf(cardClass, domainStart + cardClass.length)
    const domainCard = html.slice(domainStart, localStart)
    const localCard = html.slice(localStart)

    expect(domainCard).toContain('inbound-card--domain')
    expect(domainCard).toContain('Hub Managed')
    expect(domainCard).toContain('Domain inbound managed by Hub')
    expect(domainCard).toContain('Managed by Hub domain inbound group')
    expect(localCard).not.toContain('inbound-card--domain')
    expect(localCard).not.toContain('Hub Managed')
    expect(localCard).not.toContain('Domain inbound managed by Hub')
    expect(localCard).not.toContain('Managed by Hub domain inbound group')
  })

  it('keeps delete available for Hub-managed inbounds', async () => {
    const html = await renderInbounds()

    const cardClass = 'app-entity-card inbound-card'
    const domainStart = html.indexOf(cardClass)
    const localStart = html.indexOf(cardClass, domainStart + cardClass.length)
    const domainCard = html.slice(domainStart, localStart)

    expect(domainCard).toContain('mdi-file-edit')
    expect(domainCard).toContain('mdi-file-remove')
    expect(domainCard).toContain('mdi-content-duplicate')
  })
})
