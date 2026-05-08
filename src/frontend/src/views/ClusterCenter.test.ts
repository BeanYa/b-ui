import { readFileSync } from 'node:fs'
import { fileURLToPath } from 'node:url'
import { describe, expect, it } from 'vitest'

describe('ClusterCenter view source', () => {
  it('loads domains and members from cluster APIs and exposes register, sync, and delete actions', () => {
    const source = readFileSync(fileURLToPath(new URL('./ClusterCenter.vue', import.meta.url)), 'utf8')

    expect(source).toContain("HttpUtils.get('api/cluster/domains')")
    expect(source).toContain("HttpUtils.get('api/cluster/members')")
    expect(source).toContain('listDomainResources')
    expect(source).toContain("HttpUtils.post('api/cluster/register'")
    expect(source).toContain("HttpUtils.post('api/cluster/sync'")
    expect(source).toContain('HttpUtils.post(`api/cluster/domains/${domain.id}/update-check`, {})')
    expect(source).toContain('HttpUtils.post(`api/cluster/members/${member.id}/panel-update`, {')
    expect(source).toContain("HttpUtils.delete(`api/cluster/members/${member.id}${force ? '?force=1' : ''}`)")
    expect(source).toContain("HttpUtils.delete(`api/cluster/domains/${domain.id}${force ? '?force=1' : ''}`)")
    expect(source).toContain("HttpUtils.get(`api/cluster/operations/${operationId}`)")
  })

  it('offers a second force-delete confirmation after cluster delete failures', () => {
    const source = readFileSync(fileURLToPath(new URL('./ClusterCenter.vue', import.meta.url)), 'utf8')

    expect(source).toContain('const pendingForceDelete = ref(false)')
    expect(source).toContain("message.includes('delete cluster member')")
    expect(source).toContain("message.includes('leave cluster domain')")
    expect(source).toContain('requestDeleteMember(member, true)')
    expect(source).toContain('requestLeaveDomain(true)')
    expect(source).toContain("pendingForceDelete ? $t('clusterCenter.actions.forceDelete')")
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

  it('keeps the loading mask up for the initial cluster sync until page data is ready', () => {
    const source = readFileSync(fileURLToPath(new URL('./ClusterCenter.vue', import.meta.url)), 'utf8')
    const mountedStart = source.indexOf('onMounted(async () => {')
    const syncIndex = source.indexOf('await syncClusterState()', mountedStart)
    const finallyIndex = source.indexOf('} finally {', mountedStart)
    const pageLoadingStartIndex = source.indexOf('pageLoading.value = true', mountedStart)
    const globalLoadingStartIndex = source.indexOf('globalLoading.value = true', mountedStart)
    const pageLoadingStopIndex = source.indexOf('pageLoading.value = false', finallyIndex)
    const globalLoadingStopIndex = source.indexOf('globalLoading.value = false', finallyIndex)

    expect(source).toContain("inject<Ref<boolean>>('loading', ref(false))")
    expect(source).toContain('const pageLoading = ref(true)')
    expect(mountedStart).toBeGreaterThan(-1)
    expect(syncIndex).toBeGreaterThan(mountedStart)
    expect(finallyIndex).toBeGreaterThan(syncIndex)
    expect(pageLoadingStartIndex).toBeGreaterThan(mountedStart)
    expect(globalLoadingStartIndex).toBeGreaterThan(mountedStart)
    expect(pageLoadingStartIndex).toBeLessThan(syncIndex)
    expect(globalLoadingStartIndex).toBeLessThan(syncIndex)
    expect(pageLoadingStopIndex).toBeGreaterThan(syncIndex)
    expect(globalLoadingStopIndex).toBeGreaterThan(syncIndex)
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

  it('shows domain update policy and manual update availability in the detail metadata', () => {
    const source = readFileSync(fileURLToPath(new URL('./ClusterCenter.vue', import.meta.url)), 'utf8')

    expect(source).toContain("$t('clusterCenter.fields.updatePolicy')")
    expect(source).toContain('formatDomainUpdatePolicy(selectedDomain.updatePolicy)')
    expect(source).toContain('manualUpdateAvailable(selectedDomain)')
    expect(source).toContain("$t('clusterCenter.updateAvailable')")
    expect(source).toContain('const domainUpdateChecks = ref<Record<number, ClusterPanelUpdateCheck>>({})')
    expect(source).toContain('await checkDomainPanelUpdate(domain)')
  })

  it('marks domain resources as Domain-managed and exposes create/retry status controls in domain detail', () => {
    const source = readFileSync(fileURLToPath(new URL('./ClusterCenter.vue', import.meta.url)), 'utf8')

    expect(source).toContain('createDomainUserResource')
    expect(source).toContain('DomainResourceInboundEditor')
    expect(source).toContain('DomainResourceUserEditor')
    expect(source).toContain("$t('clusterCenter.domainResources.title')")
    expect(source).toContain("$t('clusterCenter.domainResources.domainManaged')")
    expect(source).toContain("$t('clusterCenter.domainResources.createInbound')")
    expect(source).toContain("$t('clusterCenter.domainResources.createUser')")
    expect(source).toContain("$t('clusterCenter.domainResources.retry')")
    expect(source).toContain('@click="openDomainInboundResourceDialog"')
    expect(source).toContain('@click="openDomainUserResourceDialog"')
    expect(source).toContain('const openDomainInboundResourceDialog = () => {')
    expect(source).toContain('const openDomainUserResourceDialog = () => {')
    expect(source).toContain(':members="domainInboundTargetMembers"')
    expect(source).toContain('const domainInboundTargetMembers = computed(() =>')
    expect(source).toContain('selectedDomainMembers.value.some((member) => member.isLocal)')
    expect(source).toContain('@submit="submitDomainInboundResource"')
    expect(source).toContain('@submit="submitDomainUserResource"')
    expect(source).toContain('const submitDomainInboundResource = async (payload: CreateDomainInboundResourcePayload) => {')
    expect(source).toContain('await createDomainInboundResource(selectedDomain.value.id, {')
    expect(source).toContain('...payload')
    expect(source).toContain('const submitDomainUserResource = async (payload: CreateDomainUserResourcePayload) => {')
    expect(source).toContain('await createDomainUserResource(selectedDomain.value.id, {')
    expect(source).toContain('const retryLastDomainResourceOperation = async () => {')
    expect(source).toContain('await retryDomainResourceOperation(lastDomainResourceOperation.value.operationId)')
    expect(source).not.toContain('@click="createDomainInboundFromDetail"')
    expect(source).not.toContain('const parseDomainResourceJson =')
    expect(source).not.toContain('const DOMAIN_INBOUND_TYPE_OPTIONS =')
    expect(source).not.toContain('v-model="domainInboundForm.type"')
    expect(source).not.toContain('v-model="domainInboundForm.advancedJson"')
    expect(source).not.toContain('const buildDomainInboundPayload =')
    expect(source).not.toContain('parseDomainResourceJson(domainInboundForm.value.inboundJson')
    expect(source).not.toContain('parseDomainResourceJson(domainUserForm.value.configJson')
    expect(source).not.toContain('domainInboundForm.enableTls')
    expect(source).not.toContain('Hub-managed')
    expect(source).not.toContain('Hub managed')
  })

  it('lists domain inbound groups and wires edit/delete resource operations from the detail panel', () => {
    const source = readFileSync(fileURLToPath(new URL('./ClusterCenter.vue', import.meta.url)), 'utf8')

    expect(source).toContain('selectedDomainInboundResources')
    expect(source).toContain('v-for="inbound in selectedDomainInboundResources"')
    expect(source).toContain('@click="openDomainInboundEditDialog(inbound)"')
    expect(source).toContain('@click="deleteDomainInboundGroup(inbound.group_id)"')
    expect(source).toContain('updateDomainInboundResource')
    expect(source).toContain('deleteDomainInboundResource')
    expect(source).toContain(':mode="domainInboundDialogMode"')
    expect(source).toContain(':initial-resource="editingDomainInboundResource"')
    expect(source).toContain('const openDomainInboundEditDialog = (inbound: DomainResourceInboundView) => {')
    expect(source).toContain('await updateDomainInboundResource(selectedDomain.value.id, editingGroupId, {')
    expect(source).toContain('await deleteDomainInboundResource(selectedDomain.value.id, groupId)')
    expect(source).toContain('await refreshDomainResourceGroups([selectedDomain.value.id])')
  })

  it('lists domain users beside inbound resources and wires edit/delete user operations from the detail panel', () => {
    const source = readFileSync(fileURLToPath(new URL('./ClusterCenter.vue', import.meta.url)), 'utf8')

    expect(source).toContain('selectedDomainUserResources')
    expect(source).toContain('v-for="user in selectedDomainUserResources"')
    expect(source).toContain('@click="openDomainUserEditDialog(user)"')
    expect(source).toContain('@click="deleteDomainUser(user.uuid)"')
    expect(source).toContain('updateDomainUserResource')
    expect(source).toContain('deleteDomainUserResource')
    expect(source).toContain(':mode="domainUserDialogMode"')
    expect(source).toContain(':initial-resource="editingDomainUserResource"')
    expect(source).toContain('const openDomainUserEditDialog = (user: DomainResourceUserView) => {')
    expect(source).toContain('await updateDomainUserResource(selectedDomain.value.id, editingUserUuid, {')
    expect(source).toContain('await deleteDomainUserResource(selectedDomain.value.id, userUUID)')
    expect(source).toContain('resources.domain_users')
  })

  it('separates inbound and user resource groups and deduplicates domain users by uuid while preserving applied nodes for editing', () => {
    const source = readFileSync(fileURLToPath(new URL('./ClusterCenter.vue', import.meta.url)), 'utf8')

    expect(source).toContain("$t('clusterCenter.domainResources.inboundsSection')")
    expect(source).toContain("$t('clusterCenter.domainResources.usersSection')")
    expect(source).toContain('const mergeDomainUserResources = (users: DomainResourceUserView[]): DomainResourceUserView[] => {')
    expect(source).toContain('const userMap = new Map<string, DomainResourceUserView>()')
    expect(source).toContain('applied_nodes')
    expect(source).toContain('domainUserAppliedNodeLabel')
    expect(source).toContain('<span class="cluster-center__domain-resource-node-count">')
    expect(source).toContain('[domainId, inbounds, mergeDomainUserResources(users),')
    expect(source).not.toContain('[domainId, resources.domain_inbounds, resources.domain_users,')
  })

  it('passes available domain inbound groups into the domain user editor and keeps the latest group selected by default', () => {
    const source = readFileSync(fileURLToPath(new URL('./ClusterCenter.vue', import.meta.url)), 'utf8')

    expect(source).toContain(':available-inbound-groups="availableDomainInboundGroups"')
    expect(source).toContain(':default-inbound-group="selectedDomainDefaultInboundGroup"')
    expect(source).toContain('const availableDomainInboundGroups = computed(() =>')
    expect(source).toContain('const selectedDomainDefaultInboundGroup = computed(() =>')
    expect(source).toContain('await refreshDomainResourceGroups(domains.value.map((domain) => domain.id))')
    expect(source).toContain('const resources = await listDomainResources(domainId)')
    expect(source).toContain('inbounds.map((inbound) => inbound.group_id)')
    expect(source).not.toContain('remote_inbound_id')
  })

  it('keeps domain inbound group selections scoped to the selected domain', () => {
    const source = readFileSync(fileURLToPath(new URL('./ClusterCenter.vue', import.meta.url)), 'utf8')

    expect(source).toContain('const lastDomainInboundGroupIdByDomain = ref<Record<number, string>>({})')
    expect(source).toContain('const domainInboundGroupIdsByDomain = ref<Record<number, string[]>>({})')
    expect(source).toContain('lastDomainInboundGroupIdByDomain.value[selectedDomain.value.id]')
    expect(source).toContain('domainInboundGroupIdsByDomain.value[selectedDomain.value.id] ?? []')
    expect(source).toContain('[selectedDomain.value.id]: payload.group_id')
    expect(source).not.toContain("const lastDomainInboundGroupId = ref('')")
    expect(source).not.toContain('const domainInboundGroupIds = ref<string[]>([])')
  })

  it('surfaces domain resource operation instance errors instead of only showing failed status', () => {
    const source = readFileSync(fileURLToPath(new URL('./ClusterCenter.vue', import.meta.url)), 'utf8')

    expect(source).toContain('const domainOperationInstanceErrors = computed(() =>')
    expect(source).toContain('lastDomainResourceOperation.value?.instances')
    expect(source).toContain('class="cluster-center__operation-errors"')
    expect(source).toContain("$t('clusterCenter.domainResources.operationErrors')")
    expect(source).toContain('instance.error')
  })

  it('renders member panel versions as update buttons with warning marks when outdated', () => {
    const source = readFileSync(fileURLToPath(new URL('./ClusterCenter.vue', import.meta.url)), 'utf8')

    expect(source).toContain('class="cluster-center__panel-version-button"')
    expect(source).toContain(':class="panelVersionButtonClass(member)"')
    expect(source).toContain('@click="openPanelUpdateDialog(member)"')
    expect(source).toContain("memberPanelVersionState(member) === 'outdated'")
    expect(source).toContain('<template v-if="memberPanelVersionState(member) === \'outdated\'"> ⚠</template>')
    expect(source).toContain('memberPanelVersionState(member)')
    expect(source).toContain('effectiveDomainLatestPanelVersion(selectedDomain.value)')
    expect(source).toContain('comparePanelVersions')
  })

  it('confirms targeted panel updates and disables rows while update status is pending', () => {
    const source = readFileSync(fileURLToPath(new URL('./ClusterCenter.vue', import.meta.url)), 'utf8')

    expect(source).toContain('const panelUpdateDialog = ref(false)')
    expect(source).toContain('const selectedPanelUpdateMember = ref<ClusterMember | null>(null)')
    expect(source).toContain('const panelUpdatePending = ref<Record<number, boolean>>({})')
    expect(source).toContain('const openPanelUpdateDialog = (member: ClusterMember) => {')
    expect(source).toContain('const confirmPanelUpdate = async () => {')
    expect(source).toContain('panelUpdatePending.value = { ...panelUpdatePending.value, [member.id]: true }')
    expect(source).toContain('memberPanelUpdateDisabled(member)')
    expect(source).toContain("member.status === 'updating'")
    expect(source).toContain("member.status === 'updating' ? 'orange' : member.status === 'offline' ? 'red' : 'green'")
  })

  it('keeps member version, panel version, status, latency, and action columns aligned', () => {
    const source = readFileSync(fileURLToPath(new URL('./ClusterCenter.vue', import.meta.url)), 'utf8')
    const memberSectionStart = source.indexOf('selectedDomainMembers.length === 0')
    const tableStart = source.indexOf('<table class="cluster-center__member-table">', memberSectionStart)
    const tableEnd = source.indexOf('</table>', tableStart)
    const tableSource = source.slice(tableStart, tableEnd)

    const expectedHeaderOrder = [
      "$t('clusterCenter.table.version')",
      "$t('clusterCenter.table.panelVersion')",
      "$t('clusterCenter.table.status')",
      "$t('clusterCenter.table.latency')",
      "$t('clusterCenter.table.action')",
    ]
    const expectedCellOrder = [
      'formatClusterVersionLabel(member.lastVersion)',
      'formatPanelVersion(member.panelVersion)',
      'memberStatusColor(member)',
      'memberLatency(member.nodeId)',
      'member.isLocal ? requestLeaveDomain() : requestDeleteMember(member)',
    ]

    let lastIndex = -1
    for (const token of expectedHeaderOrder) {
      const index = tableSource.indexOf(token)
      expect(index).toBeGreaterThan(lastIndex)
      lastIndex = index
    }

    lastIndex = -1
    for (const token of expectedCellOrder) {
      const index = tableSource.indexOf(token)
      expect(index).toBeGreaterThan(lastIndex)
      lastIndex = index
    }
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

  it('keeps the subscription node address separate from the panel BaseURL during registration', () => {
    const source = readFileSync(fileURLToPath(new URL('./ClusterCenter.vue', import.meta.url)), 'utf8')

    expect(source).toContain('address: \'\'')
    expect(source).toContain('const resolveNodeAddress = () =>')
    expect(source).toContain('Data().subURI')
    expect(source).toContain('return new URL(Data().subURI).hostname.toLowerCase()')
    expect(source).toContain("v-model=\"form.address\"")
    expect(source).toContain("$t('clusterCenter.fields.nodeAddress')")
    expect(source).toContain('address: resolveNodeAddress()')
    expect(source).toContain('{{ confirmInfo.address }}')
    expect(source).toContain('address: confirmInfo.value.address')
    expect(source).toContain("i18n.global.t('clusterCenter.validation.nodeAddress')")
    expect(source).not.toContain('address: panelBaseUrl')
  })

  it('uses the canonical Hub join URI id query parameter when registering from URI', () => {
    const source = readFileSync(fileURLToPath(new URL('./ClusterCenter.vue', import.meta.url)), 'utf8')

    expect(source).toContain('placeholder="buihub://hub.example.com/domain?id=example.com&domain_token=..."')
    expect(source).toContain('domain: parsed.domainId')
    expect(source).not.toContain('domain: parsed.domain,')
  })

  it('closes the display-name entry dialog and starts loading before submitting confirmed registration', () => {
    const source = readFileSync(fileURLToPath(new URL('./ClusterCenter.vue', import.meta.url)), 'utf8')
    const submitStart = source.indexOf('const confirmAndSubmit = async () => {')
    const requestIndex = source.indexOf("const registerMsg = await HttpUtils.post('api/cluster/register'", submitStart)
    const loadingIndex = source.indexOf('actionLoading.value = true', submitStart)
    const entryDialogCloseIndex = source.indexOf('registerDialog.value = false', submitStart)
    const confirmDialogCloseIndex = source.indexOf('confirmDialog.value = false', submitStart)

    expect(submitStart).toBeGreaterThan(-1)
    expect(requestIndex).toBeGreaterThan(submitStart)
    expect(loadingIndex).toBeGreaterThan(submitStart)
    expect(entryDialogCloseIndex).toBeGreaterThan(submitStart)
    expect(confirmDialogCloseIndex).toBeGreaterThan(submitStart)
    expect(loadingIndex).toBeLessThan(requestIndex)
    expect(entryDialogCloseIndex).toBeLessThan(requestIndex)
    expect(confirmDialogCloseIndex).toBeGreaterThan(requestIndex)
  })

  it('places the leave-domain action inside domain details instead of the global toolbar', () => {
    const source = readFileSync(fileURLToPath(new URL('./ClusterCenter.vue', import.meta.url)), 'utf8')

    const toolbarStart = source.indexOf('<div class="app-page__toolbar-actions cluster-center__actions">')
    const toolbarEnd = source.indexOf('</div>', toolbarStart)
    const toolbarSource = source.slice(toolbarStart, toolbarEnd)

    expect(toolbarSource).not.toContain("{{ $t('clusterCenter.actions.leave') }}")
    expect(source).toContain('cluster-center__detail-actions')
    expect(source).toContain('@click="requestLeaveDomain()"')
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
    expect(source).toContain("member.isLocal ? requestLeaveDomain() : requestDeleteMember(member)")
    expect(source).toContain("member.isLocal ? $t('clusterCenter.actions.leave') : $t('clusterCenter.actions.delete')")
    expect(source).toContain('member.isLocal ? leavingDomainId === selectedDomain?.id : deletingMemberId === member.id')
  })

  it('shows a confirmation dialog before deleting a member or leaving a domain', () => {
    const source = readFileSync(fileURLToPath(new URL('./ClusterCenter.vue', import.meta.url)), 'utf8')

    expect(source).toContain('const confirmActionDialog = ref(false)')
    expect(source).toContain("const pendingAction = ref<'delete' | 'leave' | null>(null)")
    expect(source).toContain('const requestDeleteMember = (member: ClusterMember, force = false) => {')
    expect(source).toContain('const requestLeaveDomain = (force = false) => {')
    expect(source).toContain('const confirmAction = async () => {')
    expect(source).toContain("$t('clusterCenter.confirmDeleteTitle')")
    expect(source).toContain("$t('clusterCenter.confirmDeleteMember')")
    expect(source).toContain("$t('clusterCenter.confirmLeaveTitle')")
    expect(source).toContain("$t('clusterCenter.confirmLeaveDomain')")
    expect(source).toContain("$t('clusterCenter.actions.confirmDelete')")
  })

  it('opens node management with id query only so connection details are resolved server-side', () => {
    const source = readFileSync(fileURLToPath(new URL('./ClusterCenter.vue', import.meta.url)), 'utf8')

    expect(source).toContain("query: { id: member.nodeId }")
    expect(source).not.toContain("params: { nodeId: member.nodeId }")
    expect(source).not.toContain("query: { node_id: member.nodeId }")
    expect(source).not.toContain('getPeerToken')
    expect(source).not.toContain('token: getPeerToken(member)')
    expect(source).not.toContain('baseUrl: member.baseUrl')
  })

  it('streams Ping All results into the member latency table as each node result arrives', () => {
    const source = readFileSync(fileURLToPath(new URL('./ClusterCenter.vue', import.meta.url)), 'utf8')

    expect(source).toContain('await pingStore.triggerMeshPingStream(selectedDomain.value.domain, result => {')
    expect(source).toContain('meshPingResults.value = upsertMeshPairResult(meshPingResults.value, result)')
    expect(source).toContain('function upsertMeshPairResult(results: MeshPairResult[], result: MeshPairResult): MeshPairResult[]')
  })

  it('renders cluster logs oldest-to-newest and keeps refreshes anchored to the latest entry at the bottom', () => {
    const source = readFileSync(fileURLToPath(new URL('./ClusterCenter.vue', import.meta.url)), 'utf8')
    const loadStart = source.indexOf('async function loadClusterLogs()')
    const loadEnd = source.indexOf('function startClusterLogPoll()', loadStart)
    const loadSource = source.slice(loadStart, loadEnd)

    expect(loadStart).toBeGreaterThan(-1)
    expect(loadSource).toContain('clusterLogs.value = [...msg.obj].reverse()')
    expect(loadSource).toContain('scrollLogToBottom()')
    expect(loadSource).not.toContain('scrollLogToTop()')
    expect(source).toContain('function scrollLogToBottom()')
    expect(source).toContain('const atBottom = el.scrollHeight - el.scrollTop - el.clientHeight < 5')
    expect(source).toContain('el.scrollTop = el.scrollHeight')
    expect(source).not.toContain('el.scrollTop = 0')
  })

  it('refreshes cluster member state after Ping All finishes', () => {
    const source = readFileSync(fileURLToPath(new URL('./ClusterCenter.vue', import.meta.url)), 'utf8')
    const functionStart = source.indexOf('async function pingAllDomainMembers()')
    const pingIndex = source.indexOf('await pingStore.triggerMeshPingStream', functionStart)
    const syncIndex = source.indexOf('await syncClusterState()', pingIndex)

    expect(functionStart).toBeGreaterThan(-1)
    expect(pingIndex).toBeGreaterThan(functionStart)
    expect(syncIndex).toBeGreaterThan(pingIndex)
  })
})
