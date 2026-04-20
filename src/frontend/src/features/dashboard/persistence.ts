export const parseReloadItems = (value: string | null | undefined): string[] => {
  if (!value) return []

  return value
    .split(',')
    .map(item => item.trim())
    .filter((item, index, items) => item.length > 0 && items.indexOf(item) === index)
}

export const serializeReloadItems = (items: string[]): string | null => {
  const normalized = parseReloadItems(items.join(','))
  return normalized.length > 0 ? normalized.join(',') : null
}

export const hasLiveTiles = (items: string[]): boolean => parseReloadItems(items.join(',')).length > 0

export const mergeTilesData = (
  current: Record<string, any> | undefined,
  next: Record<string, any> | undefined,
): Record<string, any> => ({
  ...(current ?? {}),
  ...(next ?? {}),
})
