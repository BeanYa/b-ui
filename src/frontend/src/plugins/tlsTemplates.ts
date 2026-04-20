import RandomUtil from '@/plugins/randomUtil'
import type { tls } from '@/types/tls'

export type TlsPresetKey = 'standard' | 'hysteria2' | 'reality'

const clone = <T>(value: T): T => JSON.parse(JSON.stringify(value))

export const getTlsPresetBaseName = (preset: TlsPresetKey): string => {
  switch (preset) {
    case 'standard':
      return 'tls-template'
    case 'hysteria2':
      return 'hysteria2-template'
    case 'reality':
      return 'reality-template'
  }
}

export const ensureUniqueTlsName = (baseName: string, existingNames: string[]): string => {
  if (!existingNames.includes(baseName)) {
    return baseName
  }

  let suffix = 2
  let nextName = `${baseName}-${suffix}`
  while (existingNames.includes(nextName)) {
    suffix += 1
    nextName = `${baseName}-${suffix}`
  }

  return nextName
}

const presets: Record<TlsPresetKey, Omit<tls, 'name' | 'id'>> = {
  standard: {
    server: {
      enabled: true,
      server_name: '',
      alpn: ['h2', 'http/1.1'],
      min_version: '1.2',
      max_version: '1.3',
      certificate_path: '',
      key_path: '',
    },
    client: {},
  },
  hysteria2: {
    server: {
      enabled: true,
      server_name: '',
      alpn: ['h3'],
      min_version: '1.3',
      max_version: '1.3',
      certificate_path: '',
      key_path: '',
    },
    client: {},
  },
  reality: {
    server: {
      enabled: true,
      server_name: 'www.youtube.com',
      reality: {
        enabled: true,
        handshake: {
          server: 'www.youtube.com',
          server_port: 443,
        },
        private_key: '',
        short_id: RandomUtil.randomShortId(),
        max_time_difference: '1m',
      },
    },
    client: {
      utls: {
        enabled: true,
        fingerprint: 'chrome',
      },
      reality: {
        enabled: true,
        public_key: '',
        short_id: '',
      },
    },
  },
}

export const createTlsPreset = (preset: TlsPresetKey, name?: string): tls => {
  const base = clone(presets[preset])

  return {
    id: 0,
    name: name ?? getTlsPresetBaseName(preset),
    server: base.server,
    client: base.client,
  }
}
