type Translate = (key: string) => string

export function createClientTableHeaders(t: Translate) {
  return [
    { title: t('client.name'), key: 'name' },
    { title: t('client.desc'), key: 'desc' },
    { title: t('client.group'), key: 'group' },
    { title: t('pages.inbounds'), key: 'inbounds', width: '7.5rem' },
    { title: t('actions.action'), key: 'actions', sortable: false },
    { title: t('stats.volume'), key: 'volume' },
    { title: t('date.expiry'), key: 'expiry' },
    { title: t('online'), key: 'online' },
    { key: 'data-table-group', width: 0 },
  ]
}
