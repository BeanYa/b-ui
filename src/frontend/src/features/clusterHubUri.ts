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

export const parseClusterHubJoinUri = (uri: string): ClusterHubJoinUri | null => {
  const trimmed = uri.trim()
  if (!trimmed.startsWith('buihub://')) return null
  if (/^buihub:\/\/https?:\/\//i.test(trimmed)) return null

  try {
    const url = new URL(trimmed)
    const domainMatch = url.pathname.match(/^\/domain\/(.+)$/i) || url.pathname.match(/^\/([^/]+)$/i)
    if (!domainMatch) return null
    const token = url.searchParams.get('domain_token') || url.searchParams.get('token') || ''
    if (!token) return null

    return {
      domain: decodeURIComponent(domainMatch[1]),
      host: url.host,
      protocol: protocolFromParams(url.searchParams, url.hostname),
      token,
    }
  } catch {
    return null
  }
}
