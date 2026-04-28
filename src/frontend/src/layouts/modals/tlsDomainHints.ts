export interface DomainHintItem {
  domain: string
  status: string
  tlsVersion?: string
  alpn?: string
  redirect?: boolean
  latencyMs?: number
  error?: string
}

export interface DomainHintDisplayItem {
  value: string
  domain: string
  metaLabels: string[]
}

export const normalizeDomainSelection = (value: unknown): string => {
  if (typeof value === 'string') {
    return value.trim()
  }

  if (value && typeof value === 'object') {
    const item = value as Record<string, unknown>
    if (typeof item.value === 'string') {
      return item.value.trim()
    }
    if (typeof item.domain === 'string') {
      return item.domain.trim()
    }
  }

  return ''
}

export const buildDomainHintItems = (
  items: DomainHintItem[],
  t: (key: string) => string,
): DomainHintDisplayItem[] => {
  return items.map((item) => {
    const metaLabels: string[] = []

    if (item.status) metaLabels.push(t(`tls.status.${item.status}`))
    if (item.tlsVersion) metaLabels.push(item.tlsVersion)
    if (item.alpn) metaLabels.push(item.alpn.toUpperCase())
    if (item.redirect) metaLabels.push(t('tls.redirected'))
    if (item.latencyMs) metaLabels.push(`${item.latencyMs}ms`)

    return {
      value: item.domain,
      domain: item.domain,
      metaLabels,
    }
  })
}
