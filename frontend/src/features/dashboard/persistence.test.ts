import { describe, expect, it } from 'vitest'

import { hasLiveTiles, mergeTilesData, parseReloadItems, serializeReloadItems } from './persistence'

describe('parseReloadItems', () => {
  it('drops empty storage entries', () => {
    expect(parseReloadItems('g-cpu,,h-net,')).toEqual(['g-cpu', 'h-net'])
  })
})

describe('serializeReloadItems', () => {
  it('returns null when nothing is selected', () => {
    expect(serializeReloadItems([])).toBeNull()
  })
})

describe('hasLiveTiles', () => {
  it('treats an empty selection as hidden tiles section', () => {
    expect(hasLiveTiles([])).toBe(false)
    expect(hasLiveTiles(['g-cpu'])).toBe(true)
  })
})

describe('mergeTilesData', () => {
  it('keeps existing runtime blocks when a tile refresh omits them', () => {
    expect(mergeTilesData(
      {
        sys: { hostName: 'node-a' },
        sbd: { running: true },
      },
      {
        cpu: 42,
      },
    )).toEqual({
      sys: { hostName: 'node-a' },
      sbd: { running: true },
      cpu: 42,
    })
  })
})
