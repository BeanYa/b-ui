import type { oTls, tls } from '@/types/tls'
import { createTlsPreset, type TlsPresetKey } from '@/plugins/tlsTemplates'

export interface TlsPresetMaterialProvider {
  generateTlsKeypair(serverName: string): Promise<string[]>
  generateRealityKeypair(): Promise<string[]>
  getTlsDomainHints?(): Promise<unknown[]>
}

const clone = <T>(value: T): T => JSON.parse(JSON.stringify(value))

const normalizeServerName = (serverName?: string): string => {
  const normalized = serverName?.trim()
  return normalized && normalized.length > 0 ? normalized : "''"
}

const normalizeOptionalServerName = (serverName: unknown): string => {
  if (typeof serverName === 'string') {
    return serverName.trim()
  }

  if (serverName && typeof serverName === 'object') {
    const item = serverName as Record<string, unknown>
    if (typeof item.value === 'string') {
      return item.value.trim()
    }
    if (typeof item.domain === 'string') {
      return item.domain.trim()
    }
  }

  return ''
}

const firstTlsDomainHint = async (provider: TlsPresetMaterialProvider): Promise<string> => {
  const hints = await provider.getTlsDomainHints?.()
  if (!Array.isArray(hints)) {
    return ''
  }

  for (const hint of hints) {
    const domain = normalizeOptionalServerName(hint)
    if (domain.length > 0) {
      return domain
    }
  }

  return ''
}

const isPemBoundary = (line: string, marker: string, boundary: 'BEGIN' | 'END'): boolean => {
  return line.startsWith(`-----${boundary} `) && line.endsWith(` ${marker}-----`)
}

const parseGeneratedTlsKeypair = (lines: string[]): { certificate: string[], key: string[] } => {
  const certificate: string[] = []
  const key: string[] = []
  let activeBlock: 'certificate' | 'key' | undefined

  for (const rawLine of lines) {
    const line = String(rawLine)
    if (isPemBoundary(line, 'CERTIFICATE', 'BEGIN')) {
      activeBlock = 'certificate'
      certificate.push(line)
      continue
    }
    if (isPemBoundary(line, 'CERTIFICATE', 'END')) {
      certificate.push(line)
      activeBlock = undefined
      continue
    }
    if (line.startsWith('-----BEGIN ') && line.endsWith(' PRIVATE KEY-----')) {
      activeBlock = 'key'
      key.push(line)
      continue
    }
    if (line.startsWith('-----END ') && line.endsWith(' PRIVATE KEY-----')) {
      key.push(line)
      activeBlock = undefined
      continue
    }
    if (activeBlock === 'certificate') {
      certificate.push(line)
    } else if (activeBlock === 'key') {
      key.push(line)
    }
  }

  if (certificate.length === 0 || key.length === 0) {
    throw new Error('Failed to parse generated TLS keypair')
  }

  return { certificate, key }
}

const parseGeneratedRealityKeypair = (lines: string[]): { privateKey: string, publicKey: string } => {
  let privateKey = ''
  let publicKey = ''

  for (const rawLine of lines) {
    const line = String(rawLine)
    if (line.startsWith('PrivateKey:')) {
      privateKey = line.slice('PrivateKey:'.length).trim()
    }
    if (line.startsWith('PublicKey:')) {
      publicKey = line.slice('PublicKey:'.length).trim()
    }
  }

  if (!privateKey || !publicKey) {
    throw new Error('Failed to parse generated Reality keypair')
  }

  return { privateKey, publicKey }
}

const ensureRealityClient = (client: oTls): NonNullable<oTls['reality']> => {
  if (client.reality == null) {
    client.reality = {
      enabled: true,
      public_key: '',
      short_id: '',
    }
  }
  return client.reality
}

const getDefaultMaterialProvider = async (): Promise<TlsPresetMaterialProvider> => {
  const { default: HttpUtils } = await import('@/plugins/httputil')

  return {
    async generateTlsKeypair(serverName: string) {
      const { default: Data } = await import('@/store/modules/data')
      const keypairs = await Data().keypairs('tls', serverName)
      if (!Array.isArray(keypairs) || keypairs.length === 0) {
        throw new Error('Failed to generate TLS keypair')
      }
      return keypairs
    },
    async generateRealityKeypair() {
      const { default: Data } = await import('@/store/modules/data')
      const keypairs = await Data().keypairs('reality')
      if (!Array.isArray(keypairs) || keypairs.length === 0) {
        throw new Error('Failed to generate Reality keypair')
      }
      return keypairs
    },
    async getTlsDomainHints() {
      const msg = await HttpUtils.get('api/domainHints')
      if (!msg.success || !Array.isArray(msg.obj?.items)) {
        return []
      }
      return msg.obj.items as unknown[]
    },
  }
}

export const materializeTlsPreset = async (
  preset: TlsPresetKey,
  value: tls,
  provider?: TlsPresetMaterialProvider,
): Promise<tls> => {
  const resolvedProvider = provider ?? await getDefaultMaterialProvider()
  const next = clone(value)

  switch (preset) {
    case 'standard':
    case 'hysteria2': {
      const serverName = normalizeOptionalServerName(next.server.server_name)
      if (serverName.length > 0) {
        next.server.server_name = serverName
      } else if (preset === 'hysteria2') {
        const hintedServerName = await firstTlsDomainHint(resolvedProvider)
        if (hintedServerName.length > 0) {
          next.server.server_name = hintedServerName
        }
      }
      const material = parseGeneratedTlsKeypair(
        await resolvedProvider.generateTlsKeypair(normalizeServerName(next.server.server_name)),
      )
      next.server.certificate = material.certificate
      next.server.key = material.key
      delete next.server.certificate_path
      delete next.server.key_path
      return next
    }
    case 'reality': {
      const material = parseGeneratedRealityKeypair(await resolvedProvider.generateRealityKeypair())
      if (next.server.reality == null) {
        throw new Error('Reality preset is missing server reality settings')
      }
      next.server.reality.private_key = material.privateKey
      ensureRealityClient(next.client).public_key = material.publicKey
      return next
    }
  }
}

export const createMaterializedTlsPreset = async (
  preset: TlsPresetKey,
  name?: string,
  provider?: TlsPresetMaterialProvider,
): Promise<tls> => {
  return materializeTlsPreset(preset, createTlsPreset(preset, name), provider)
}
