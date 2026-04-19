export type TileLayoutGroups = {
  metric: string[]
  detail: string[]
}

export const splitTileItemsByLayout = (items: string[]): TileLayoutGroups => items.reduce<TileLayoutGroups>((groups, item) => {
  if (item.startsWith('g')) {
    groups.metric.push(item)
    return groups
  }

  groups.detail.push(item)
  return groups
}, {
  metric: [],
  detail: [],
})
