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
