import { describe, expect, it } from 'vitest'

import { createDefaultConfig, normalizeConfig, normalizeOnlines } from './normalize'

describe('normalizeOnlines', () => {
  it('fills missing online buckets with empty arrays', () => {
    expect(normalizeOnlines({ user: ['alice'] })).toEqual({
      inbound: [],
      outbound: [],
      user: ['alice'],
    })
  })
})

describe('normalizeConfig', () => {
  it('supplies required nested config sections when payload is partial', () => {
    const config = normalizeConfig({
      log: { disabled: true },
      route: { final: 'direct' },
    })

    expect(config.log.disabled).toBe(true)
    expect(config.dns.servers).toEqual([])
    expect(config.dns.rules).toEqual([])
    expect(config.route.rules).toEqual([])
    expect(config.route.rule_set).toEqual([])
    expect(config.route.final).toBe('direct')
    expect(config.experimental).toEqual({})
  })

  it('creates a safe default config shape', () => {
    const config = createDefaultConfig()

    expect(config.log).toEqual({})
    expect(config.dns.servers).toEqual([])
    expect(config.route.rules).toEqual([])
    expect(config.experimental).toEqual({})
  })
})
