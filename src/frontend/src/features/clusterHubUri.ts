export type ClusterHubJoinUri = {
  domain: string
  host: string
  protocol: 'http' | 'https'
  token: string
}

const localHosts = new Set(['localhost', '127.0.0.1', '::1'])

const protocolFromParams = (params: URLSearchParams, hostname: string): 'http' | 'https' => {
  const explicitProtocol = params.get('hub_protocol') || params.get('protocol')
  if (explicitProtocol === 'http' || explicitProtocol === 'https') {
    return explicitProtocol
  }
  return localHosts.has(hostname) ? 'http' : 'https'
}

const firstParam = (params: URLSearchParams, names: string[]) => {
  for (const name of names) {
    const value = params.get(name)
    if (value) return value
  }
  return ''
}

export const parseClusterHubJoinUri = (uri: string): ClusterHubJoinUri | null => {
  const trimmed = uri.trim()
  if (!trimmed.startsWith('buihub://')) return null
  if (/^buihub:\/\/https?:\/\//i.test(trimmed)) return null

  try {
    const url = new URL(trimmed)
    const domainMatch = url.pathname.match(/^\/domain\/(.+)$/i) || url.pathname.match(/^\/([^/]+)$/i)
    const domain = domainMatch?.[1] || firstParam(url.searchParams, ['domain_id', 'domainId', 'domain'])
    if (!domain) return null
    const token = firstParam(url.searchParams, ['domain_token', 'domainToken', 'domain-token', 'token'])
    if (!token) return null

    return {
      domain: decodeURIComponent(domain),
      host: url.host,
      protocol: protocolFromParams(url.searchParams, url.hostname),
      token,
    }
  } catch {
    return null
  }
}
