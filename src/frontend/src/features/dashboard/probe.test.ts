import { describe, expect, it } from 'vitest'

import { filterTileSectionsByData, formatAppVersion, formatCpuRingNote } from './probe'

describe('formatAppVersion', () => {
  it('preserves versions that already include a v prefix', () => {
    expect(formatAppVersion('v0.0.2')).toBe('v0.0.2')
  })

  it('adds a v prefix when the payload omits it', () => {
    expect(formatAppVersion('0.0.2')).toBe('v0.0.2')
  })
})

describe('formatCpuRingNote', () => {
  it('shows a compact core-count summary for the probe ring', () => {
    expect(formatCpuRingNote(1, '核心')).toBe('1核心')
    expect(formatCpuRingNote(2, '核心')).toBe('2核心')
    expect(formatCpuRingNote(4, 'core')).toBe('4 core')
  })
})

describe('filterTileSectionsByData', () => {
  it('hides the reserve layer when swap data is unavailable', () => {
    expect(filterTileSectionsByData([
      { key: 'metric', items: ['g-swp'] },
      { key: 'detail', items: ['h-net'] },
    ], {})).toEqual([
      { key: 'detail', items: ['h-net'] },
    ])
  })

  it('keeps the reserve layer when swap telemetry is present', () => {
    expect(filterTileSectionsByData([
      { key: 'metric', items: ['g-swp'] },
      { key: 'detail', items: ['h-net'] },
    ], {
      swp: { current: 0, total: 1024 },
    })).toEqual([
      { key: 'metric', items: ['g-swp'] },
      { key: 'detail', items: ['h-net'] },
    ])
  })
})
