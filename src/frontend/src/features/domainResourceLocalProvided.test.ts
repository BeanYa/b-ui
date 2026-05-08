import { beforeAll, describe, expect, it } from 'vitest'

let helpers: typeof import('./domainResourceLocalProvided')

beforeAll(async () => {
  Object.defineProperty(globalThis, 'window', {
    value: {
      crypto: {
        getRandomValues(values: Uint8Array | Uint32Array) {
          values[0] = 1
          values[1] = 1
          return values
        },
      },
    },
    configurable: true,
  })
  helpers = await import('./domainResourceLocalProvided')
})

describe('domain resource LocalProvided helpers', () => {
  it('builds the marker object expected by target nodes', () => {
    const { localProvided } = helpers

    expect(localProvided('DomainInboundListenPort')).toEqual({
      LocalProvided: 'DomainInboundListenPort',
    })
  })

  it('creates generated TLS templates with target-node domain and key material', () => {
    const { createDomainInboundTls } = helpers

    expect(createDomainInboundTls('standard', 'edge-main', 'example.com')).toEqual({
      tls_template: 'standard',
      tls: {
        name: 'edge-main-tls',
        server: {
          enabled: true,
          server_name: { LocalProvided: 'DomainName' },
          alpn: ['h2', 'http/1.1'],
          certificate: { LocalProvided: 'GeneratedTLSCertificate' },
          key: { LocalProvided: 'GeneratedTLSKey' },
        },
        client: { insecure: true },
      },
    })
  })

  it('creates panel-certificate TLS templates with target-node panel settings', () => {
    const { createDomainInboundTls } = helpers

    expect(createDomainInboundTls('standard-cert', 'edge-main', 'example.com')).toEqual({
      tls_template: 'standard-cert',
      tls: {
        name: 'edge-main-tls',
        server: {
          enabled: true,
          server_name: { LocalProvided: 'PanelWebDomain' },
          alpn: ['h2', 'http/1.1'],
          certificate_path: { LocalProvided: 'PanelWebCertFile' },
          key_path: { LocalProvided: 'PanelWebKeyFile' },
        },
        client: {},
      },
    })
  })

  it('creates Reality TLS templates with target-node key generation markers', () => {
    const { createDomainInboundTls } = helpers
    const payload = createDomainInboundTls('reality', 'edge-main', 'example.com')

    expect(payload?.tls_template).toBe('reality')
    expect(payload?.tls?.server.reality).toMatchObject({
      enabled: true,
      private_key: { LocalProvided: 'RealityPrivateKey' },
      short_id: '',
    })
    expect(payload?.tls?.client.reality).toMatchObject({
      enabled: true,
      public_key: { LocalProvided: 'RealityPublicKey' },
    })
  })

  it('returns no TLS payload when the domain inbound does not request TLS', () => {
    const { createDomainInboundTls } = helpers

    expect(createDomainInboundTls('none', 'edge-main', 'example.com')).toEqual({})
  })

  it('creates user protocol config with target-node provided secrets by default', () => {
    const { createDomainUserConfig } = helpers

    const config = createDomainUserConfig('Alice', {
      uuid: 'auto',
      password: 'auto',
      auth: 'auto',
    })

    expect(config.vless.uuid).toEqual({ LocalProvided: 'DomainUserUUID' })
    expect(config.vmess.uuid).toEqual({ LocalProvided: 'DomainUserUUID' })
    expect(config.tuic.uuid).toEqual({ LocalProvided: 'DomainUserUUID' })
    expect(config.trojan.password).toEqual({ LocalProvided: 'DomainUserPassword' })
    expect(config.hysteria.auth_str).toEqual({ LocalProvided: 'DomainUserAuth' })
    expect(config.vless.name).toBe('Alice')
  })

  it('keeps manually provided user secrets when requested', () => {
    const { createDomainUserConfig } = helpers
    const sources: import('./domainResourceLocalProvided').DomainUserSecretSources = {
      uuid: 'manual',
      password: 'manual',
      auth: 'manual',
    }
    const config = createDomainUserConfig('Alice', sources, {
      uuid: 'manual-uuid',
      password: 'manual-password',
      auth: 'manual-auth',
    })

    expect(config.vless.uuid).toBe('manual-uuid')
    expect(config.trojan.password).toBe('manual-password')
    expect(config.hysteria.auth_str).toBe('manual-auth')
  })

  it('defines editable fields for each generated domain user protocol', () => {
    const { domainUserProtocolFields } = helpers

    expect(domainUserProtocolFields.map((item) => item.protocol)).toEqual([
      'mixed',
      'socks',
      'http',
      'shadowsocks',
      'shadowsocks16',
      'shadowtls',
      'vmess',
      'vless',
      'anytls',
      'trojan',
      'naive',
      'hysteria',
      'tuic',
      'hysteria2',
    ])
    expect(domainUserProtocolFields.find((item) => item.protocol === 'vless')?.fields).toContainEqual({
      key: 'flow',
      label: 'Flow',
      type: 'select',
      items: ['', 'xtls-rprx-vision'],
    })
    expect(domainUserProtocolFields.find((item) => item.protocol === 'vmess')?.fields).toContainEqual({
      key: 'alterId',
      label: 'Alter ID',
      type: 'number',
    })
  })

  it('sanitizes tag parts while preserving readable defaults', () => {
    const { sanitizeDomainResourcePart } = helpers

    expect(sanitizeDomainResourcePart('edge main/prod', 'fallback')).toBe('edge-main-prod')
    expect(sanitizeDomainResourcePart('  ', 'fallback')).toBe('fallback')
  })
})
