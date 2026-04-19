import type { Config } from '@/types/config'

export interface OnlineBuckets {
  inbound: string[]
  outbound: string[]
  user: string[]
}

const asArray = <T>(value: unknown): T[] => Array.isArray(value) ? value as T[] : []

const asStringArray = (value: unknown): string[] =>
  asArray<unknown>(value).filter((item): item is string => typeof item === 'string' && item.length > 0)

export const createEmptyOnlines = (): OnlineBuckets => ({
  inbound: [],
  outbound: [],
  user: [],
})

export const normalizeOnlines = (value: unknown): OnlineBuckets => {
  const source = value && typeof value === 'object' ? value as Record<string, unknown> : {}

  return {
    inbound: asStringArray(source.inbound),
    outbound: asStringArray(source.outbound),
    user: asStringArray(source.user),
  }
}

export const createDefaultConfig = (): Config => ({
  log: {},
  dns: {
    servers: [],
    rules: [],
  } as Config['dns'],
  inbounds: [],
  outbounds: [],
  route: {
    rules: [],
    rule_set: [],
    default_domain_resolver: '',
  },
  experimental: {},
})

export const normalizeCollection = <T>(value: unknown): T[] => asArray<T>(value)

export const normalizeConfig = (value: unknown): Config => {
  const source = value && typeof value === 'object' ? value as Record<string, any> : {}
  const defaults = createDefaultConfig()
  const dns = source.dns && typeof source.dns === 'object' ? source.dns : {}
  const route = source.route && typeof source.route === 'object' ? source.route : {}
  const experimental = source.experimental && typeof source.experimental === 'object' ? source.experimental : {}

  return {
    ...defaults,
    ...source,
    log: source.log && typeof source.log === 'object' ? { ...source.log } : {},
    dns: {
      ...dns,
      servers: normalizeCollection(dns.servers),
      rules: normalizeCollection(dns.rules),
    } as Config['dns'],
    inbounds: normalizeCollection(source.inbounds),
    outbounds: normalizeCollection(source.outbounds),
    route: {
      ...defaults.route,
      ...route,
      rules: normalizeCollection(route.rules),
      rule_set: normalizeCollection(route.rule_set),
    },
    experimental: {
      ...experimental,
    },
  }
}
