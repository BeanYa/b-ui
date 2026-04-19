export type TileSectionKey = 'metric' | 'detail'

export type TileSection = {
  key: TileSectionKey
  items: string[]
}

const joinCountAndUnit = (count: number, unit: string) => {
  if (!unit) return String(count)
  return /^[A-Za-z]/.test(unit) ? `${count} ${unit}` : `${count}${unit}`
}

export const formatAppVersion = (version?: string | null) => {
  const normalized = String(version ?? '').trim()

  if (!normalized) return 'Version pending'
  return normalized.startsWith('v') ? normalized : `v${normalized}`
}

export const formatCpuRingNote = (cpuCount?: number | null, coreUnit = '') => {
  if (!cpuCount) return 'Core count pending'
  return joinCountAndUnit(cpuCount, coreUnit)
}

const tileHasRenderableData = (itemKey: string, tilesData: any) => {
  if (itemKey === 'g-swp') {
    return typeof tilesData?.swp?.total === 'number' && tilesData.swp.total > 0
  }

  return true
}

export const filterTileSectionsByData = (sections: TileSection[], tilesData: any): TileSection[] =>
  sections
    .map(section => ({
      ...section,
      items: section.items.filter(item => tileHasRenderableData(item, tilesData)),
    }))
    .filter(section => section.items.length > 0)
