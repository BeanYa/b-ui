import { describe, expect, it } from 'vitest'

import { parseClusterHubJoinUri } from './clusterHubUri'

describe('parseClusterHubJoinUri', () => {
  it('parses Hub join URIs without an embedded https scheme', () => {
    expect(parseClusterHubJoinUri('buihub://hub.example.com/domain/whoisbean.com?domain_token=abc123&hub_protocol=https')).toEqual({
      domain: 'whoisbean.com',
      host: 'hub.example.com',
      protocol: 'https',
      token: 'abc123',
    })
  })

  it('keeps ports and accepts the legacy token parameter during rollout', () => {
    expect(parseClusterHubJoinUri('buihub://localhost:8787/domain/dev.example.com?token=legacy')).toEqual({
      domain: 'dev.example.com',
      host: 'localhost:8787',
      protocol: 'http',
      token: 'legacy',
    })
  })

  it('accepts direct domain paths for compact copied URIs', () => {
    expect(parseClusterHubJoinUri('buihub://hub.example.com/whoisbean.com?domain_token=compact')).toEqual({
      domain: 'whoisbean.com',
      host: 'hub.example.com',
      protocol: 'https',
      token: 'compact',
    })
  })

  it('rejects URIs that embed another URL scheme after buihub', () => {
    expect(parseClusterHubJoinUri('buihub://https://hub.example.com/domain/whoisbean.com?domain_token=abc123')).toBeNull()
  })
})
