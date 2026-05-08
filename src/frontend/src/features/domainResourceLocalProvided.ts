import { randomConfigs } from '@/types/clients'
import { createTlsPreset, type TlsPresetKey } from '@/plugins/tlsTemplates'

export type LocalProvidedKind =
  | 'DomainInboundListenPort'
  | 'DomainName'
  | 'GeneratedTLSCertificate'
  | 'GeneratedTLSKey'
  | 'PanelWebDomain'
  | 'PanelWebCertFile'
  | 'PanelWebKeyFile'
  | 'RealityPrivateKey'
  | 'RealityPublicKey'
  | 'DomainUserUUID'
  | 'DomainUserPassword'
  | 'DomainUserAuth'

export type DomainInboundTlsTemplate = TlsPresetKey | 'none'
export type DomainUserSecretSource = 'auto' | 'manual'
export type DomainUserConfig = ReturnType<typeof randomConfigs>
export type DomainUserProtocolFieldType = 'text' | 'number' | 'select'

export interface LocalProvidedValue {
  LocalProvided: LocalProvidedKind
}

export interface DomainInboundTlsPayload {
  tls_template?: TlsPresetKey
  tls?: {
    name: string
    server: Record<string, unknown>
    client: Record<string, unknown>
  }
}

export type DomainUserSecretSources = {
  uuid: DomainUserSecretSource
  password: DomainUserSecretSource
  auth: DomainUserSecretSource
}

export type DomainUserManualSecrets = {
  uuid?: string
  password?: string
  auth?: string
}

export interface DomainUserProtocolField {
  key: string
  label: string
  type?: DomainUserProtocolFieldType
  secret?: keyof DomainUserManualSecrets
  items?: string[]
}

export interface DomainUserProtocolDefinition {
  protocol: string
  fields: DomainUserProtocolField[]
}

export const domainUserProtocolFields: DomainUserProtocolDefinition[] = [
  { protocol: 'mixed', fields: [{ key: 'username', label: 'Username' }, { key: 'password', label: 'Password', secret: 'password' }] },
  { protocol: 'socks', fields: [{ key: 'username', label: 'Username' }, { key: 'password', label: 'Password', secret: 'password' }] },
  { protocol: 'http', fields: [{ key: 'username', label: 'Username' }, { key: 'password', label: 'Password', secret: 'password' }] },
  { protocol: 'shadowsocks', fields: [{ key: 'name', label: 'Name' }, { key: 'password', label: 'Password', secret: 'password' }] },
  { protocol: 'shadowsocks16', fields: [{ key: 'name', label: 'Name' }, { key: 'password', label: 'Password', secret: 'password' }] },
  { protocol: 'shadowtls', fields: [{ key: 'name', label: 'Name' }, { key: 'password', label: 'Password', secret: 'password' }] },
  { protocol: 'vmess', fields: [{ key: 'name', label: 'Name' }, { key: 'uuid', label: 'UUID', secret: 'uuid' }, { key: 'alterId', label: 'Alter ID', type: 'number' }] },
  { protocol: 'vless', fields: [{ key: 'name', label: 'Name' }, { key: 'uuid', label: 'UUID', secret: 'uuid' }, { key: 'flow', label: 'Flow', type: 'select', items: ['', 'xtls-rprx-vision'] }] },
  { protocol: 'anytls', fields: [{ key: 'name', label: 'Name' }, { key: 'password', label: 'Password', secret: 'password' }] },
  { protocol: 'trojan', fields: [{ key: 'name', label: 'Name' }, { key: 'password', label: 'Password', secret: 'password' }] },
  { protocol: 'naive', fields: [{ key: 'username', label: 'Username' }, { key: 'password', label: 'Password', secret: 'password' }] },
  { protocol: 'hysteria', fields: [{ key: 'name', label: 'Name' }, { key: 'auth_str', label: 'Auth', secret: 'auth' }] },
  { protocol: 'tuic', fields: [{ key: 'name', label: 'Name' }, { key: 'uuid', label: 'UUID', secret: 'uuid' }, { key: 'password', label: 'Password', secret: 'password' }] },
  { protocol: 'hysteria2', fields: [{ key: 'name', label: 'Name' }, { key: 'password', label: 'Password', secret: 'password' }] },
]

const clone = <T>(value: T): T => JSON.parse(JSON.stringify(value))

export const cloneDomainUserConfig = (config: DomainUserConfig): DomainUserConfig => clone(config)

export const localProvided = (kind: LocalProvidedKind): LocalProvidedValue => ({
  LocalProvided: kind,
})

export const isLocalProvided = (value: unknown): value is LocalProvidedValue => {
  if (!value || typeof value !== 'object' || Array.isArray(value)) {
    return false
  }
  return typeof (value as Record<string, unknown>).LocalProvided === 'string'
}

export const sanitizeDomainResourcePart = (value: string, fallback: string): string => {
  const normalized = value.trim().replace(/[^A-Za-z0-9_.-]/g, '-').replace(/-+/g, '-').replace(/^-|-$/g, '')
  return normalized || fallback
}

const domainTlsName = (tagSeed: string): string => `${sanitizeDomainResourcePart(tagSeed, 'domain-inbound')}-tls`

export const createDomainInboundTls = (
  template: DomainInboundTlsTemplate,
  tagSeed: string,
  domainName: string,
): DomainInboundTlsPayload => {
  if (template === 'none') {
    return {}
  }

  const preset = createTlsPreset(template, domainTlsName(tagSeed))
  const server = clone(preset.server) as Record<string, unknown>
  const client = clone(preset.client) as Record<string, unknown>

  switch (template) {
    case 'standard':
    case 'hysteria2':
      server.server_name = localProvided('DomainName')
      server.certificate = localProvided('GeneratedTLSCertificate')
      server.key = localProvided('GeneratedTLSKey')
      delete server.certificate_path
      delete server.key_path
      break
    case 'standard-cert':
    case 'hysteria2-cert':
      server.server_name = localProvided('PanelWebDomain')
      server.certificate_path = localProvided('PanelWebCertFile')
      server.key_path = localProvided('PanelWebKeyFile')
      delete server.certificate
      delete server.key
      break
    case 'reality': {
      const realityServer = {
        ...((server.reality as Record<string, unknown> | undefined) ?? {}),
        enabled: true,
        private_key: localProvided('RealityPrivateKey'),
        short_id: '',
      }
      const realityClient = {
        ...((client.reality as Record<string, unknown> | undefined) ?? {}),
        enabled: true,
        public_key: localProvided('RealityPublicKey'),
      }
      server.server_name = typeof server.server_name === 'string' && server.server_name.trim()
        ? server.server_name
        : domainName.trim() || 'www.youtube.com'
      server.reality = realityServer
      client.reality = realityClient
      break
    }
  }

  return {
    tls_template: template,
    tls: {
      name: preset.name,
      server,
      client,
    },
  }
}

export const createDomainUserConfig = (
  userName: string,
  sources: DomainUserSecretSources,
  manualSecrets: DomainUserManualSecrets = {},
): DomainUserConfig => {
  const config = clone(randomConfigs(userName))
  const uuid = sources.uuid === 'auto'
    ? localProvided('DomainUserUUID')
    : manualSecrets.uuid?.trim() || ''
  const password = sources.password === 'auto'
    ? localProvided('DomainUserPassword')
    : manualSecrets.password?.trim() || ''
  const auth = sources.auth === 'auto'
    ? localProvided('DomainUserAuth')
    : manualSecrets.auth?.trim() || ''

  for (const item of Object.values(config)) {
    if (Object.hasOwn(item, 'uuid')) {
      item.uuid = uuid
    }
    if (Object.hasOwn(item, 'password')) {
      item.password = password
    }
    if (Object.hasOwn(item, 'auth_str')) {
      item.auth_str = auth
    }
  }

  return config
}
