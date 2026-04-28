import { describe, expect, it } from 'vitest'

import { buildDomainHintItems, normalizeDomainSelection } from './tlsDomainHints'

describe('buildDomainHintItems', () => {
  it('keeps the saved value as the domain and moves diagnostics into labels', () => {
    const items = buildDomainHintItems(
      [
        {
          domain: 'example.com',
          status: 'recommended',
          tlsVersion: 'TLS 1.3',
          alpn: 'h2',
          latencyMs: 8,
        },
      ],
      (key) => key === 'tls.status.recommended' ? 'Recommended' : key,
    )

    expect(items).toEqual([
      {
        value: 'example.com',
        domain: 'example.com',
        metaLabels: ['Recommended', 'TLS 1.3', 'H2', '8ms'],
      },
    ])
  })

  it('normalizes Vuetify combobox domain objects to a plain string value', () => {
    expect(normalizeDomainSelection({
      value: 'example.com',
      domain: 'Example',
      metaLabels: ['Recommended'],
    })).toBe('example.com')
  })
})
