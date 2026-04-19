import { describe, expect, it } from 'vitest'

import { splitTileItemsByLayout } from './layout'

describe('splitTileItemsByLayout', () => {
  it('keeps gauge tiles in the compact grid and preserves original order', () => {
    expect(splitTileItemsByLayout([
      'g-swp',
      'h-cpu',
      'g-mem',
      'i-sys',
      'h-net',
      'g-cpu',
      'i-sbd',
    ])).toEqual({
      metric: ['g-swp', 'g-mem', 'g-cpu'],
      detail: ['h-cpu', 'i-sys', 'h-net', 'i-sbd'],
    })
  })
})
