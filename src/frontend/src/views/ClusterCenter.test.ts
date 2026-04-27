import { readFileSync } from 'node:fs'
import { fileURLToPath } from 'node:url'
import { describe, expect, it } from 'vitest'

describe('ClusterCenter view source', () => {
  it('loads domains and members from cluster APIs and exposes register, sync, and delete actions', () => {
    const source = readFileSync(fileURLToPath(new URL('./ClusterCenter.vue', import.meta.url)), 'utf8')

    expect(source).toContain("HttpUtils.get('api/cluster/domains')")
    expect(source).toContain("HttpUtils.get('api/cluster/members')")
    expect(source).toContain("HttpUtils.post('api/cluster/register'")
    expect(source).toContain("HttpUtils.post('api/cluster/sync'")
    expect(source).toContain("HttpUtils.delete(`api/cluster/members/${member.id}`)")
    expect(source).toContain("HttpUtils.delete(`api/cluster/domains/${domain.id}`)")
    expect(source).toContain("HttpUtils.get(`api/cluster/operations/${operationId}`)")
  })

  it('auto-syncs saved domain mirrors when the cluster center opens and surfaces cleanup messages', () => {
    const source = readFileSync(fileURLToPath(new URL('./ClusterCenter.vue', import.meta.url)), 'utf8')

    expect(source).toContain('const syncClusterState = async () => {')
    expect(source).toContain("const msg = await HttpUtils.post('api/cluster/sync', {})")
    expect(source).toContain('const operation = msg.obj as ClusterOperationStatus | null')
    expect(source).toContain('if (operation?.message) {')
    expect(source).toContain("push.error({ title: i18n.global.t('failed'), message: operation.message })")
    expect(source).toContain('await loadData()')
    expect(source).toContain('onMounted(async () => {')
    expect(source).toContain('await syncClusterState()')
  })

  it('filters member rows by the selected domain and keeps the page admin-oriented', () => {
    const source = readFileSync(fileURLToPath(new URL('./ClusterCenter.vue', import.meta.url)), 'utf8')

    expect(source).toContain('const selectedDomainId = ref<number | null>(null)')
    expect(source).toContain('const selectedDomainMembers = computed(() =>')
    expect(source).toContain('member.domainId === selectedDomainId.value')
    expect(source).toContain("$t('pages.clusterCenter')")
    expect(source).toContain("$t('clusterCenter.actions.manualSync')")
    expect(source).toContain("$t('clusterCenter.actions.register')")
    expect(source).toContain("$t('clusterCenter.actions.leave')")
    expect(source).toContain('formatClusterVersionLabel')
    expect(source).toContain('cluster-center__selected-version')
    expect(source).not.toContain('`v${selectedDomain.lastVersion}`')
    expect(source).not.toContain('v{{ domain.lastVersion }}')
    expect(source).not.toContain('v{{ member.lastVersion }}')
    expect(source).toContain('isUsableAbsoluteUrl')
    expect(source).toContain('resolvePanelBaseUrl')
    expect(source).toContain('window.location.origin')
    expect(source).toContain("i18n.global.t('clusterCenter.validation.required')")
    expect(source).toContain("i18n.global.t('clusterCenter.validation.hubUrl')")
    expect(source).toContain("i18n.global.t('clusterCenter.validation.panelUrl')")
    expect(source).not.toContain('v-model="form.baseUrl"')
    expect(source).not.toContain('v-model="form.name"')
  })

  it('uses the existing control-surface visual language instead of a generic table-only page', () => {
    const source = readFileSync(fileURLToPath(new URL('./ClusterCenter.vue', import.meta.url)), 'utf8')

    expect(source).toContain('app-page__hero')
    expect(source).toContain('app-card-shell')
    expect(source).toContain('cluster-center__domains')
    expect(source).toContain('cluster-center__members')
    expect(source).not.toContain('v-data-table')
  })

  it('renders refresh as a distinct secondary toolbar button instead of a plain text action', () => {
    const source = readFileSync(fileURLToPath(new URL('./ClusterCenter.vue', import.meta.url)), 'utf8')

    expect(source).toContain('class="cluster-center__refresh-btn" variant="outlined"')
    expect(source).toContain('background: linear-gradient(')
  })

  it('keeps the hub URL protocol selector compact so the URI host input gets the remaining width', () => {
    const source = readFileSync(fileURLToPath(new URL('./ClusterCenter.vue', import.meta.url)), 'utf8')

    expect(source).toContain('flex: 0 0 72px;')
    expect(source).toContain('max-width: 72px;')
    expect(source).toContain('min-width: 72px;')
    expect(source).toContain('flex: 1 1 auto;')
    expect(source).toContain('box-shadow: 0 0 0 3px color-mix(in srgb, var(--app-state-info) 15%, transparent);')
  })

  it('adds dedicated spacing to the confirm registration dialog title', () => {
    const source = readFileSync(fileURLToPath(new URL('./ClusterCenter.vue', import.meta.url)), 'utf8')

    expect(source).toContain('class="app-card-shell cluster-center__confirm-card"')
    expect(source).toContain('class="cluster-center__confirm-title"')
    expect(source).toContain('padding: 24px 24px 10px;')
    expect(source).toContain('padding-top: 8px;')
  })

  it('shows a single domain-list card on the center page and opens domain details explicitly', () => {
    const source = readFileSync(fileURLToPath(new URL('./ClusterCenter.vue', import.meta.url)), 'utf8')

    expect(source).toContain('v-if="!selectedDomain"')
    expect(source).toContain('class="cluster-center__domains app-card-shell"')
    expect(source).toContain('class="cluster-center__domain-prompt"')
    expect(source).toContain('@click="openDomainDetail(domain)"')
    expect(source).toContain('const openDomainDetail = (domain: ClusterDomain) => {')
    expect(source).not.toContain("selectedDomainId.value = domains.value[0]?.id ?? null")
  })

  it('renders a detail state with back navigation, domain metadata rows, a dedicated action tree rail, and registered cluster servers', () => {
    const source = readFileSync(fileURLToPath(new URL('./ClusterCenter.vue', import.meta.url)), 'utf8')

    expect(source).toContain('v-else class="cluster-center__detail"')
    expect(source).toContain('@click="backToClusterCenter"')
    expect(source).toContain('ClusterDomainActionTree')
    expect(source).toContain('class="cluster-center__detail-panel"')
    expect(source).toContain('class="cluster-center__domain-meta"')
    expect(source).toContain('class="cluster-center__actions-tree"')
    expect(source).toContain("{{ $t('clusterCenter.registeredServers') }}")
    expect(source).toContain('const backToClusterCenter = () => {')
  })

  it('deduplicates registration checks by normalized BaseURL and defaults display name from BaseURL host', () => {
    const source = readFileSync(fileURLToPath(new URL('./ClusterCenter.vue', import.meta.url)), 'utf8')

    expect(source).toContain('const normalizeClusterBaseUrl = (value: string) =>')
    expect(source).toContain('const deriveDisplayNameFromBaseUrl = (baseUrl: string) =>')
    expect(source).toContain("match(/^https?:\\/\\/([^/:?#]+)(?::\\d+)?(?:[/?#]|$)/i)")
    expect(source).toContain('displayName: deriveDisplayNameFromBaseUrl(panelBaseUrl)')
    expect(source).toContain('normalizeClusterBaseUrl(m.base_url || m.baseUrl || \'\') === normalizedPanelBaseUrl')
    expect(source).toContain('form.value.displayName = confirmInfo.value.displayName')
  })

  it('places the leave-domain action inside domain details instead of the global toolbar', () => {
    const source = readFileSync(fileURLToPath(new URL('./ClusterCenter.vue', import.meta.url)), 'utf8')

    const toolbarStart = source.indexOf('<div class="app-page__toolbar-actions cluster-center__actions">')
    const toolbarEnd = source.indexOf('</div>', toolbarStart)
    const toolbarSource = source.slice(toolbarStart, toolbarEnd)

    expect(toolbarSource).not.toContain("{{ $t('clusterCenter.actions.leave') }}")
    expect(source).toContain('cluster-center__detail-actions')
    expect(source).toContain('@click="leaveDomain(selectedDomain)"')
    expect(source).toContain(':loading="leavingDomainId === selectedDomain.id"')
  })

  it('renders supported actions through the dedicated tree component instead of a flat joined string', () => {
    const source = readFileSync(fileURLToPath(new URL('./ClusterCenter.vue', import.meta.url)), 'utf8')

    expect(source).toContain(':supported-actions="selectedDomain.supportedActions"')
    expect(source).toContain("{{ $t('clusterCenter.fields.supportedActions') }}")
    expect(source).toContain('cluster-center__meta-row')
    expect(source).not.toContain('formatSupportedActions(selectedDomain.supportedActions)')
    expect(source).not.toContain('const formatSupportedActions =')
  })

  it('marks the local member and uses leave-domain semantics for its row action', () => {
    const source = readFileSync(fileURLToPath(new URL('./ClusterCenter.vue', import.meta.url)), 'utf8')

    expect(source).toContain('member.isLocal')
    expect(source).toContain("{{ $t('clusterCenter.localNode') }}")
    expect(source).toContain("member.isLocal ? leaveDomain(selectedDomain) : deleteMember(member)")
    expect(source).toContain("member.isLocal ? $t('clusterCenter.actions.leave') : $t('clusterCenter.actions.delete')")
    expect(source).toContain('member.isLocal ? leavingDomainId === selectedDomain?.id : deletingMemberId === member.id')
  })

  it('opens node management with node_id only so connection details are resolved server-side', () => {
    const source = readFileSync(fileURLToPath(new URL('./ClusterCenter.vue', import.meta.url)), 'utf8')

    expect(source).toContain("query: { node_id: member.nodeId }")
    expect(source).not.toContain('getPeerToken')
    expect(source).not.toContain('token: getPeerToken(member)')
    expect(source).not.toContain('baseUrl: member.baseUrl')
  })
})
