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

type DomainUserConfig = ReturnType<typeof randomConfigs>

const clone = <T>(value: T): T => JSON.parse(JSON.stringify(value))

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
