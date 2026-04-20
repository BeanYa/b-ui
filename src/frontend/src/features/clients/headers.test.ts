import { describe, expect, it } from 'vitest'

import { createClientTableHeaders } from './headers'

describe('createClientTableHeaders', () => {
  it('keeps the inbounds header wide enough to stay horizontal', () => {
    const headers = createClientTableHeaders((key: string) => key)
    const inboundHeader = headers.find(header => header.key === 'inbounds')

    expect(inboundHeader).toBeDefined()
    expect(inboundHeader?.width).toBe('7.5rem')
  })
})
