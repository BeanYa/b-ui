<template>
  <div class="app-page">
    <section class="app-page__hero cluster-center__hero">
      <div class="app-page__hero-head">
        <div class="app-page__hero-kicker">{{ $t('clusterCenter.heroKicker') }}</div>
        <h1 class="app-page__hero-title">{{ $t('pages.clusterCenter') }}</h1>
        <p class="app-page__hero-copy">
          {{ $t('clusterCenter.heroCopy') }}
        </p>
        <div class="app-page__hero-meta">
          <span class="app-page__hero-meta-item">{{ domains.length }} {{ $t('clusterCenter.domainsTitle').toLowerCase() }}</span>
          <span class="app-page__hero-meta-item">{{ members.length }} {{ $t('clusterCenter.mirroredMembers') }}</span>
          <span class="app-page__hero-meta-item">{{ selectedDomain ? formatClusterVersionLabel(selectedDomain.lastVersion) : $t('clusterCenter.metaNoDomain') }}</span>
        </div>
      </div>
    </section>

    <v-row class="app-page__toolbar">
      <v-col cols="12">
        <div class="app-page__toolbar-actions cluster-center__actions">
          <v-btn color="primary" @click="registerDialog = true">{{ $t('clusterCenter.actions.register') }}</v-btn>
          <v-btn variant="outlined" color="warning" :loading="actionLoading" @click="manualSync">{{ $t('clusterCenter.actions.manualSync') }}</v-btn>
          <v-btn class="cluster-center__refresh-btn" variant="outlined" :loading="pageLoading" @click="loadData">{{ $t('clusterCenter.actions.refresh') }}</v-btn>
        </div>
      </v-col>
    </v-row>

    <template v-if="!selectedDomain">
      <v-card class="cluster-center__domains app-card-shell" :loading="pageLoading">
        <v-card-title>
          <div class="cluster-center__card-title">
            <span>{{ $t('clusterCenter.domainsTitle') }}</span>
            <span class="cluster-center__domain-prompt">{{ $t('clusterCenter.inspectPrompt') }}</span>
          </div>
        </v-card-title>
        <v-card-text>
          <div v-if="domains.length === 0" class="cluster-center__empty">{{ $t('clusterCenter.noDomains') }}</div>
          <div v-else class="cluster-center__domain-list">
            <button
              v-for="domain in domains"
              :key="domain.id"
              type="button"
              class="cluster-center__domain-card"
              @click="openDomainDetail(domain)"
            >
              <div class="cluster-center__domain-head">
                <strong>{{ domain.domain }}</strong>
                <span class="cluster-center__version">{{ formatClusterVersionLabel(domain.lastVersion) }}</span>
              </div>
              <div class="cluster-center__domain-url">{{ domain.hubUrl || $t('clusterCenter.fields.hubUrl') }}</div>
              <div class="cluster-center__domain-meta">{{ domainMemberCount(domain.id) }} {{ $t('clusterCenter.mirroredMembers') }}</div>
            </button>
          </div>
        </v-card-text>
      </v-card>
    </template>

    <section v-else class="cluster-center__detail">
      <div class="cluster-center__detail-actions">
        <v-btn variant="outlined" prepend-icon="mdi-arrow-left" @click="backToClusterCenter">
          {{ $t('clusterCenter.actions.back') }}
        </v-btn>
        <v-btn
          variant="outlined"
          color="error"
          :loading="leavingDomainId === selectedDomain.id"
          @click="requestLeaveDomain()"
        >
          {{ $t('clusterCenter.actions.leave') }}
        </v-btn>
      </div>

      <v-card class="app-card-shell cluster-center__domain-info" :loading="pageLoading">
        <v-card-title>
          <div class="cluster-center__selected-head">
            <span>{{ selectedDomain.domain }}</span>
            <span class="cluster-center__version cluster-center__selected-version">
              {{ formatClusterVersionLabel(selectedDomain.lastVersion) }}
            </span>
          </div>
        </v-card-title>
        <v-card-text>
          <div class="cluster-center__detail-panel">
            <div class="cluster-center__domain-meta">
              <div class="cluster-center__meta-row">
                <span class="cluster-center__meta-label">{{ $t('clusterCenter.fields.domain') }}</span>
                <strong class="cluster-center__meta-value">{{ selectedDomain.domain }}</strong>
              </div>
              <div class="cluster-center__meta-row">
                <span class="cluster-center__meta-label">{{ $t('clusterCenter.fields.hubUrl') }}</span>
                <strong class="cluster-center__meta-value">{{ selectedDomain.hubUrl || '-' }}</strong>
              </div>
              <div class="cluster-center__meta-row">
                <span class="cluster-center__meta-label">{{ $t('clusterCenter.fields.updatePolicy') }}</span>
                <strong class="cluster-center__meta-value cluster-center__policy-badges">
                  <span class="cluster-center__policy-badge">
                    {{ formatDomainUpdatePolicy(selectedDomain.updatePolicy) }}
                  </span>
                  <span
                    v-if="manualUpdateAvailable(selectedDomain)"
                    class="cluster-center__update-available-badge"
                  >
                    {{ $t('clusterCenter.updateAvailable') }}
                  </span>
                </strong>
              </div>
              <div class="cluster-center__meta-row">
                <span class="cluster-center__meta-label">{{ $t('clusterCenter.table.version') }}</span>
                <strong class="cluster-center__meta-value">{{ formatClusterVersionLabel(selectedDomain.lastVersion) }}</strong>
              </div>
              <div class="cluster-center__meta-row">
                <span class="cluster-center__meta-label">{{ $t('clusterCenter.fields.communicationProtocol') }}</span>
                <strong class="cluster-center__meta-value">{{ selectedDomain.communicationProtocolVersion || '-' }}</strong>
              </div>
              <div class="cluster-center__meta-row">
                <span class="cluster-center__meta-label">{{ $t('clusterCenter.fields.communicationEndpoint') }}</span>
                <strong class="cluster-center__meta-value">{{ selectedDomain.communicationEndpointPath || '-' }}</strong>
              </div>
              <div class="cluster-center__meta-row">
                <span class="cluster-center__meta-label">{{ $t('clusterCenter.mirroredMembers') }}</span>
                <strong class="cluster-center__meta-value">{{ selectedDomainMembers.length }}</strong>
              </div>
            </div>

            <div class="cluster-center__actions-tree">
              <span class="cluster-center__meta-label cluster-center__meta-label--header">
                {{ $t('clusterCenter.fields.supportedActions') }}
              </span>
              <ClusterDomainActionTree
                :supported-actions="selectedDomain.supportedActions"
              />
            </div>
          </div>
        </v-card-text>
      </v-card>

      <v-card class="app-card-shell cluster-center__jobs" :loading="clusterStore.tasksLoading">
        <v-card-title>
          <div style="display: flex; align-items: center; gap: 16px;">
            <span>{{ $t('clusterCenter.processingJobs.title') }}</span>
            <v-btn
              size="small"
              variant="outlined"
              color="primary"
              @click="showRunTaskDialog = true"
            >
              {{ $t('clusterCenter.processingJobs.runTask') }}
            </v-btn>
          </div>
        </v-card-title>
        <v-card-text>
          <div v-if="clusterStore.tasks.length === 0" class="cluster-center__empty">
            {{ $t('clusterCenter.processingJobs.empty') }}
          </div>
          <div v-else class="cluster-center__member-table-wrap">
            <table class="cluster-center__member-table">
              <thead>
                <tr>
                  <th>{{ $t('clusterCenter.processingJobs.table.taskType') }}</th>
                  <th>{{ $t('clusterCenter.processingJobs.table.status') }}</th>
                  <th>{{ $t('clusterCenter.processingJobs.table.scope') }}</th>
                  <th>{{ $t('clusterCenter.processingJobs.table.progress') }}</th>
                  <th>{{ $t('clusterCenter.processingJobs.table.created') }}</th>
                  <th>{{ $t('clusterCenter.table.action') }}</th>
                </tr>
              </thead>
              <tbody>
                <tr v-for="task in clusterStore.tasks" :key="task.taskId">
                  <td>
                    <span class="cluster-center__task-type">{{ task.taskType }}</span>
                  </td>
                  <td>
                    <v-chip
                      :color="taskStatusColor(task.status)"
                      size="small"
                      variant="flat"
                    >
                      {{ task.status }}
                    </v-chip>
                  </td>
                  <td>{{ task.scope }}</td>
                  <td>{{ task.progress }}</td>
                  <td>{{ task.createdAt }}</td>
                  <td>
                    <v-btn
                      v-if="task.status === 'completed'"
                      size="small"
                      variant="tonal"
                      @click="goToTaskResult(task.taskId)"
                    >
                      {{ $t('clusterCenter.processingJobs.viewResult') }}
                    </v-btn>
                  </td>
                </tr>
              </tbody>
            </table>
          </div>
        </v-card-text>
      </v-card>

      <v-card class="app-card-shell cluster-center__domain-resources" :loading="domainResourceLoading">
        <v-card-title>
          <div class="cluster-center__card-title">
            <span>{{ $t('clusterCenter.domainResources.title') }}</span>
            <span class="cluster-center__domain-prompt">{{ $t('clusterCenter.domainResources.domainManaged') }}</span>
          </div>
        </v-card-title>
        <v-card-text>
          <div class="cluster-center__domain-resource-actions">
            <v-btn
              size="small"
              color="primary"
              variant="outlined"
              :loading="domainResourceLoading"
              @click="openDomainInboundResourceDialog"
            >
              {{ $t('clusterCenter.domainResources.createInbound') }}
            </v-btn>
            <v-btn
              size="small"
              color="primary"
              variant="outlined"
              :loading="domainResourceLoading"
              @click="openDomainUserResourceDialog"
            >
              {{ $t('clusterCenter.domainResources.createUser') }}
            </v-btn>
            <v-btn
              size="small"
              color="warning"
              variant="outlined"
              :disabled="!lastDomainResourceOperation"
              :loading="domainResourceLoading"
              @click="retryLastDomainResourceOperation"
            >
              {{ $t('clusterCenter.domainResources.retry') }}
            </v-btn>
          </div>
          <div v-if="lastDomainResourceOperation" class="cluster-center__operation-status">
            <span class="cluster-center__operation-status-label">
              {{ $t('clusterCenter.domainResources.lastOperation') }}
            </span>
            <span class="cluster-center__meta-label">{{ lastDomainResourceOperation.operationId }}</span>
            <v-chip size="small" :color="domainOperationStatusColor(lastDomainResourceOperation.status)" variant="flat">
              {{ formatDomainResourceOperationStatus(lastDomainResourceOperation.status) }}
            </v-chip>
          </div>
          <div v-if="domainOperationInstanceErrors.length > 0" class="cluster-center__operation-errors">
            <div class="cluster-center__operation-errors-title">
              {{ $t('clusterCenter.domainResources.operationErrors') }}
            </div>
            <div
              v-for="instance in domainOperationInstanceErrors"
              :key="instance.nodeId"
              class="cluster-center__operation-error"
            >
              <span class="cluster-center__operation-error-node">
                {{ instance.displayName || instance.nodeId }}
              </span>
              <span class="cluster-center__operation-error-message">{{ instance.error }}</span>
            </div>
          </div>
          <section v-if="selectedDomainInboundResources.length > 0" class="cluster-center__domain-resource-section">
            <div class="cluster-center__domain-resource-section-title">
              {{ $t('clusterCenter.domainResources.inboundsSection') }}
            </div>
            <div class="cluster-center__domain-inbound-list">
              <div
                v-for="inbound in selectedDomainInboundResources"
                :key="inbound.group_id"
                class="cluster-center__domain-inbound-row"
              >
                <div class="cluster-center__domain-inbound-main">
                  <span class="cluster-center__domain-inbound-group">{{ inbound.group_id }}</span>
                  <span class="cluster-center__domain-inbound-type">{{ inbound.type || '-' }}</span>
                </div>
                <div class="cluster-center__domain-inbound-actions">
                  <v-chip size="small" :color="domainInboundStatusColor(inbound.status)" variant="flat">
                    {{ inbound.status || '-' }}
                  </v-chip>
                  <v-btn
                    size="small"
                    variant="text"
                    :icon="true"
                    :title="$t('actions.edit')"
                    :disabled="domainResourceLoading"
                    @click="openDomainInboundEditDialog(inbound)"
                  >
                    <v-icon size="18">mdi-pencil</v-icon>
                  </v-btn>
                  <v-btn
                    size="small"
                    variant="text"
                    color="error"
                    :icon="true"
                    :title="$t('actions.del')"
                    :disabled="domainResourceLoading"
                    @click="requestDeleteDomainInboundGroup(inbound)"
                  >
                    <v-icon size="18">mdi-delete</v-icon>
                  </v-btn>
                </div>
              </div>
            </div>
          </section>
          <section v-if="selectedDomainUserResources.length > 0" class="cluster-center__domain-resource-section">
            <div class="cluster-center__domain-resource-section-title">
              {{ $t('clusterCenter.domainResources.usersSection') }}
            </div>
            <div class="cluster-center__domain-inbound-list">
              <div
                v-for="user in selectedDomainUserResources"
                :key="user.uuid"
                class="cluster-center__domain-inbound-row"
              >
                <div class="cluster-center__domain-inbound-main">
                  <span class="cluster-center__domain-inbound-group">{{ user.name || user.uuid }}</span>
                  <span class="cluster-center__domain-inbound-type">{{ user.uuid }}</span>
                  <span class="cluster-center__domain-resource-node-count">
                    {{ domainUserAppliedNodeLabel(user) }}
                  </span>
                </div>
                <div class="cluster-center__domain-inbound-actions">
                  <v-chip size="small" :color="user.enable ? 'green' : 'grey'" variant="flat">
                    {{ user.enable ? $t('enable') : $t('disable') }}
                  </v-chip>
                  <v-btn
                    size="small"
                    variant="text"
                    :icon="true"
                    :title="$t('actions.edit')"
                    :disabled="domainResourceLoading"
                    @click="openDomainUserEditDialog(user)"
                  >
                    <v-icon size="18">mdi-pencil</v-icon>
                  </v-btn>
                  <v-btn
                    size="small"
                    variant="text"
                    color="error"
                    :icon="true"
                    :title="$t('actions.del')"
                    :disabled="domainResourceLoading"
                    @click="requestDeleteDomainUser(user)"
                  >
                    <v-icon size="18">mdi-delete</v-icon>
                  </v-btn>
                </div>
              </div>
            </div>
          </section>
          <div v-if="!lastDomainResourceOperation" class="cluster-center__empty">
            {{ $t('clusterCenter.domainResources.noOperation') }}
          </div>
        </v-card-text>
      </v-card>

      <v-card class="app-card-shell cluster-center__logs">
        <v-card-title>
          <div class="cluster-center__card-title">
            <span>{{ $t('clusterCenter.logs.title') }}</span>
            <span class="cluster-center__log-count">{{ clusterLogs.length }}</span>
          </div>
        </v-card-title>
        <v-card-text>
          <div v-if="clusterLogs.length === 0" class="cluster-center__empty">{{ $t('clusterCenter.logs.empty') }}</div>
          <div v-else ref="logContainer" class="cluster-center__log-container" @scroll="onLogContainerScroll">
            <div v-for="(entry, idx) in clusterLogs" :key="idx" class="cluster-center__log-line" :class="'cluster-center__log-line--' + entry.level.toLowerCase()">
              <span class="cluster-center__log-time">{{ entry.time }}</span>
              <span class="cluster-center__log-dir">{{ entry.direction }}</span>
              <span class="cluster-center__log-action">{{ entry.action }}</span>
              <span class="cluster-center__log-fields">{{ formatLogFields(entry.fields) }}</span>
            </div>
          </div>
        </v-card-text>
      </v-card>

      <v-card class="app-card-shell cluster-center__members" :loading="pageLoading">
        <v-card-title>
          <div style="display: flex; align-items: center; gap: 16px;">
            <span>{{ $t('clusterCenter.registeredServers') }}</span>
            <v-btn
              size="small"
              variant="outlined"
              color="primary"
              :loading="meshPingLoading"
              @click="pingAllDomainMembers"
            >
              {{ $t('clusterCenter.actions.pingAll') }}
            </v-btn>
            <v-btn
              size="small"
              variant="outlined"
              :icon="true"
              @click="openPingSettingsDialog"
            >
              <v-icon size="18">mdi-cog</v-icon>
            </v-btn>
            <span
              v-if="pingPolicy.enabled"
              class="cluster-center__auto-ping-indicator"
              :title="`Auto ping: every ${pingPolicy.interval}s`"
            >
              <span class="cluster-center__auto-ping-dot"></span>
              <span class="cluster-center__auto-ping-text">{{ formatAutoPingTime() }}</span>
            </span>
          </div>
        </v-card-title>
        <v-card-text>
          <div v-if="selectedDomainMembers.length === 0" class="cluster-center__empty">{{ $t('clusterCenter.noMembers') }}</div>
          <div v-else class="cluster-center__member-table-wrap">
            <table class="cluster-center__member-table">
              <thead>
                <tr>
                  <th>{{ $t('clusterCenter.table.name') }}</th>
                  <th>{{ $t('clusterCenter.table.baseUrl') }}</th>
                  <th>{{ $t('clusterCenter.table.version') }}</th>
                  <th>{{ $t('clusterCenter.table.panelVersion') }}</th>
                  <th>{{ $t('clusterCenter.table.status') }}</th>
                  <th>{{ $t('clusterCenter.table.latency') }}</th>
                  <th>{{ $t('clusterCenter.table.region') }}</th>
                  <th>{{ $t('clusterCenter.table.action') }}</th>
                </tr>
              </thead>
              <tbody>
                <tr v-for="member in selectedDomainMembers" :key="member.id">
                  <td>
                    <div class="cluster-center__member-node">
                      <v-text-field
                        :model-value="member.displayName || member.name || ''"
                        density="compact"
                        variant="underlined"
                        hide-details
                        :placeholder="$t('clusterCenter.table.displayName')"
                        @update:model-value="(v: string) => onMemberDisplayNameInput(member, v)"
                        @blur="commitMemberDisplayName(member)"
                        @keydown.enter.prevent="($event.target as HTMLInputElement | null)?.blur()"
                      />
                      <span v-if="member.isLocal" class="cluster-center__local-badge">{{ $t('clusterCenter.localNode') }}</span>
                    </div>
                  </td>
                  <td>{{ member.baseUrl || '-' }}</td>
                  <td>{{ formatClusterVersionLabel(member.lastVersion) }}</td>
                  <td>
                    <v-btn
                      class="cluster-center__panel-version-button"
                      :class="panelVersionButtonClass(member)"
                      size="small"
                      variant="outlined"
                      :disabled="memberPanelUpdateDisabled(member)"
                      :loading="Boolean(panelUpdatePending[member.id])"
                      @click="openPanelUpdateDialog(member)"
                    >
                      {{ formatPanelVersion(member.panelVersion) }}
                      <template v-if="memberPanelVersionState(member) === 'outdated'"> ⚠</template>
                    </v-btn>
                  </td>
                  <td>
                    <v-chip
                      :color="memberStatusColor(member)"
                      size="small"
                      variant="flat"
                    >
                      {{ memberStatusLabel(member) }}
                    </v-chip>
                  </td>
                  <td>
                    <span
                      :style="memberLatencyStyle(member.nodeId)"
                      class="cluster-center__latency-cell"
                    >{{ memberLatency(member.nodeId) }}</span>
                  </td>
                  <td>
                    <span v-if="memberCountryDisplay(member)">{{ memberCountryDisplay(member) }}</span>
                    <span v-else>-</span>
                  </td>
                  <td>
                    <div style="display: flex; gap: 8px; align-items: center;">
                      <v-btn
                        v-if="!member.isLocal"
                        size="small"
                        variant="tonal"
                        @click="goToNodeDetail(member)"
                      >
                        {{ $t('clusterCenter.actions.manage') }}
                      </v-btn>
                      <v-btn
                        size="small"
                        :color="member.isLocal ? 'error' : 'warning'"
                        variant="outlined"
                        :loading="member.isLocal ? leavingDomainId === selectedDomain?.id : deletingMemberId === member.id"
                        @click="member.isLocal ? requestLeaveDomain() : requestDeleteMember(member)"
                      >
                        {{ member.isLocal ? $t('clusterCenter.actions.leave') : $t('clusterCenter.actions.delete') }}
                      </v-btn>
                    </div>
                  </td>
                </tr>
              </tbody>
            </table>
          </div>
        </v-card-text>
      </v-card>
    </section>

    <v-dialog v-model="registerDialog" class="app-dialog app-dialog--compact" max-width="520" @update:model-value="onRegisterDialogClose">
      <v-card class="app-card-shell">
        <v-card-title>{{ $t('clusterCenter.dialogTitle') }}</v-card-title>

        <template v-if="registerStep === 1">
          <v-tabs v-model="registerMode" class="cluster-center__register-tabs" color="primary" grow>
            <v-tab value="uri">URI</v-tab>
            <v-tab value="manual">{{ $t('clusterCenter.domainsTitle') }}</v-tab>
          </v-tabs>

          <v-card-text class="cluster-center__dialog-body">
            <template v-if="registerMode === 'uri'">
              <v-text-field
                v-model="form.joinUri"
                :label="$t('clusterCenter.fields.joinUri')"
                placeholder="buihub://hub.example.com/domain?id=example.com&domain_token=..."
                persistent-hint
                :hint="$t('clusterCenter.joinUriHint')"
              />
            </template>
            <template v-else>
              <v-text-field v-model="form.domain" :label="$t('clusterCenter.fields.domain')" hide-details />
              <div class="cluster-center__hub-url-field">
                <v-select
                  v-model="form.hubUrlProtocol"
                  :items="['https', 'http']"
                  variant="plain"
                  hide-details
                  density="compact"
                  class="cluster-center__hub-url-protocol"
                />
                <span class="cluster-center__hub-url-sep">://</span>
                <v-text-field
                  v-model="form.hubUrlHost"
                  :label="$t('clusterCenter.fields.hubUrl')"
                  hide-details
                  class="cluster-center__hub-url-host"
                />
              </div>
              <v-text-field v-model="form.token" :label="$t('clusterCenter.fields.token')" type="password" hide-details />
            </template>
          </v-card-text>
          <v-card-actions>
            <v-spacer />
            <v-btn variant="text" @click="registerDialog = false">{{ $t('clusterCenter.actions.cancel') }}</v-btn>
            <v-btn color="primary" :loading="checkingUrl" @click="validateAndCheckDomain">{{ $t('clusterCenter.actions.submit') }}</v-btn>
          </v-card-actions>
        </template>

        <template v-if="registerStep === 2">
          <v-card-text class="cluster-center__dialog-body">
            <div class="cluster-center__step-indicator">
              <span class="cluster-center__step-label">{{ $t('clusterCenter.stepDomainInfo') }}</span>
              <span class="cluster-center__step-value">{{ confirmInfo.domain }}</span>
            </div>
            <v-text-field
              v-model="form.displayName"
              :label="$t('clusterCenter.displayName')"
              :hint="$t('clusterCenter.displayNameHint')"
              persistent-hint
              hide-details
            />
            <v-text-field
              v-model="form.address"
              :label="$t('clusterCenter.fields.nodeAddress')"
              hide-details
            />
          </v-card-text>
          <v-card-actions>
            <v-spacer />
            <v-btn variant="text" @click="registerStep = 1">{{ $t('clusterCenter.actions.cancel') }}</v-btn>
            <v-btn color="primary" @click="showConfirmDialog">{{ $t('clusterCenter.actions.submit') }}</v-btn>
          </v-card-actions>
        </template>
      </v-card>
    </v-dialog>

    <v-dialog v-model="confirmDialog" class="app-dialog" max-width="520">
      <v-card class="app-card-shell cluster-center__confirm-card">
        <v-card-title class="cluster-center__confirm-title">{{ $t('clusterCenter.confirmTitle') }}</v-card-title>
        <v-card-text class="cluster-center__confirm-body">
          <div class="cluster-center__confirm-table-wrap">
            <table class="cluster-center__confirm-table">
              <tbody>
                <tr>
                  <td class="cluster-center__confirm-label">{{ $t('clusterCenter.fields.hubUrl') }}</td>
                  <td class="cluster-center__confirm-value">{{ confirmInfo.hubUrl }}</td>
                </tr>
                <tr>
                  <td class="cluster-center__confirm-label">{{ $t('clusterCenter.fields.domain') }}</td>
                  <td class="cluster-center__confirm-value">{{ confirmInfo.domain }}</td>
                </tr>
                <tr>
                  <td class="cluster-center__confirm-label">{{ $t('clusterCenter.fields.token') }}</td>
                  <td class="cluster-center__confirm-value cluster-center__confirm-token">{{ confirmInfo.token }}</td>
                </tr>
                <tr>
                  <td class="cluster-center__confirm-label">{{ $t('clusterCenter.displayName') }}</td>
                  <td class="cluster-center__confirm-value">{{ confirmInfo.displayName }}</td>
                </tr>
                <tr>
                  <td class="cluster-center__confirm-label">{{ $t('clusterCenter.fields.localBaseUrl') }}</td>
                  <td class="cluster-center__confirm-value">{{ confirmInfo.baseUrl }}</td>
                </tr>
                <tr>
                  <td class="cluster-center__confirm-label">{{ $t('clusterCenter.fields.nodeAddress') }}</td>
                  <td class="cluster-center__confirm-value">{{ confirmInfo.address }}</td>
                </tr>
              </tbody>
            </table>
          </div>
        </v-card-text>
        <v-card-actions>
          <v-spacer />
          <v-btn variant="text" @click="confirmDialog = false">{{ $t('clusterCenter.actions.cancel') }}</v-btn>
          <v-btn color="primary" :loading="actionLoading" @click="confirmAndSubmit">{{ $t('clusterCenter.actions.confirmRegister') }}</v-btn>
        </v-card-actions>
      </v-card>
    </v-dialog>

    <v-dialog v-model="alreadyExistsDialog" class="app-dialog" max-width="460">
      <v-card class="app-card-shell">
        <v-card-title>{{ $t('clusterCenter.alreadyExists') }}</v-card-title>
        <v-card-text>
          <p>{{ $t('clusterCenter.alreadyExistsHint') }}</p>
        </v-card-text>
        <v-card-actions>
          <v-spacer />
          <v-btn variant="text" @click="alreadyExistsDialog = false">{{ $t('clusterCenter.actions.cancel') }}</v-btn>
          <v-btn color="primary" :loading="actionLoading" @click="pullExistingDomain">{{ $t('clusterCenter.pullDomain') }}</v-btn>
        </v-card-actions>
      </v-card>
    </v-dialog>

    <v-dialog v-model="panelUpdateDialog" class="app-dialog app-dialog--compact" max-width="460">
      <v-card class="app-card-shell">
        <v-card-title>{{ $t('clusterCenter.panelUpdate.confirmTitle') }}</v-card-title>
        <v-card-text class="cluster-center__dialog-body">
          <div class="cluster-center__step-indicator">
            <span class="cluster-center__step-label">{{ $t('clusterCenter.table.node') }}</span>
            <span class="cluster-center__step-value">{{ selectedPanelUpdateMember?.displayName || selectedPanelUpdateMember?.name || selectedPanelUpdateMember?.nodeId }}</span>
          </div>
          <div class="cluster-center__step-indicator">
            <span class="cluster-center__step-label">{{ $t('clusterCenter.table.panelVersion') }}</span>
            <span class="cluster-center__step-value">
              {{ formatPanelVersion(selectedPanelUpdateMember?.panelVersion) }}
              -> {{ formatPanelVersion(selectedPanelUpdateTargetVersion) }}
            </span>
          </div>
          <p class="cluster-center__panel-update-copy">{{ $t('clusterCenter.panelUpdate.confirmCopy') }}</p>
        </v-card-text>
        <v-card-actions>
          <v-spacer />
          <v-btn variant="text" @click="panelUpdateDialog = false">{{ $t('clusterCenter.actions.cancel') }}</v-btn>
          <v-btn color="warning" :loading="selectedPanelUpdateMember ? Boolean(panelUpdatePending[selectedPanelUpdateMember.id]) : false" @click="confirmPanelUpdate">
            {{ $t('clusterCenter.actions.updatePanel') }}
          </v-btn>
        </v-card-actions>
      </v-card>
    </v-dialog>

    <v-dialog v-model="confirmActionDialog" class="app-dialog app-dialog--compact" max-width="460">
      <v-card class="app-card-shell">
        <v-card-title>{{ pendingForceDelete ? $t('clusterCenter.confirmForceDeleteTitle') : (pendingAction === 'leave' ? $t('clusterCenter.confirmLeaveTitle') : $t('clusterCenter.confirmDeleteTitle')) }}</v-card-title>
        <v-card-text class="cluster-center__dialog-body">
          <div class="cluster-center__step-indicator">
            <span class="cluster-center__step-label">{{ $t('clusterCenter.table.node') }}</span>
            <span class="cluster-center__step-value">{{ pendingActionTarget?.displayName || pendingActionTarget?.name || pendingActionTarget?.nodeId }}</span>
          </div>
          <p class="cluster-center__panel-update-copy">
            {{ pendingForceDelete
              ? (pendingAction === 'leave' ? $t('clusterCenter.confirmForceLeaveDomain') : $t('clusterCenter.confirmForceDeleteMember'))
              : (pendingAction === 'leave' ? $t('clusterCenter.confirmLeaveDomain') : $t('clusterCenter.confirmDeleteMember')) }}
          </p>
        </v-card-text>
        <v-card-actions>
          <v-spacer />
          <v-btn variant="text" @click="cancelPendingAction">{{ $t('clusterCenter.actions.cancel') }}</v-btn>
          <v-btn :color="pendingForceDelete || pendingAction === 'leave' ? 'error' : 'warning'" :loading="actionLoading" @click="confirmAction">
            {{ pendingForceDelete ? $t('clusterCenter.actions.forceDelete') : $t('clusterCenter.actions.confirmDelete') }}
          </v-btn>
        </v-card-actions>
      </v-card>
    </v-dialog>

    <v-dialog v-model="domainInboundDialog" class="app-dialog app-dialog--wide" max-width="1040">
      <DomainResourceInboundEditor
        v-if="selectedDomain"
        :domain="selectedDomain"
        :members="domainInboundTargetMembers"
        :loading="domainResourceLoading"
        :error="domainResourceFormError"
        :mode="domainInboundDialogMode"
        :initial-resource="editingDomainInboundResource"
        @cancel="domainInboundDialog = false"
        @submit="submitDomainInboundResource"
      />
    </v-dialog>

    <v-dialog v-model="domainUserDialog" class="app-dialog app-dialog--wide" max-width="960">
      <DomainResourceUserEditor
        v-if="selectedDomain"
        :domain="selectedDomain"
        :loading="domainResourceLoading"
        :error="domainResourceFormError"
        :available-inbound-groups="availableDomainInboundGroups"
        :default-inbound-group="selectedDomainDefaultInboundGroup"
        :mode="domainUserDialogMode"
        :initial-resource="editingDomainUserResource"
        @cancel="domainUserDialog = false"
        @submit="submitDomainUserResource"
      />
    </v-dialog>

    <v-dialog v-model="domainResourceConfirmDialog" class="app-dialog app-dialog--compact" max-width="480">
      <v-card class="app-card-shell">
        <v-card-title>{{ $t('clusterCenter.domainResources.confirmTitle') }}</v-card-title>
        <v-card-text class="cluster-center__dialog-body">
          <div class="cluster-center__step-indicator">
            <span class="cluster-center__step-label">{{ pendingDomainResourceOperationLabel }}</span>
            <span class="cluster-center__step-value">{{ pendingDomainResourceOperation?.resourceLabel || '-' }}</span>
          </div>
          <p class="cluster-center__panel-update-copy">
            {{ $t('clusterCenter.domainResources.confirmCopy') }}
          </p>
        </v-card-text>
        <v-card-actions>
          <v-spacer />
          <v-btn variant="text" @click="cancelDomainResourceOperation">
            {{ $t('clusterCenter.actions.cancel') }}
          </v-btn>
          <v-btn
            :color="pendingDomainResourceOperation?.action === 'delete' ? 'error' : 'primary'"
            :loading="domainResourceLoading"
            @click="confirmDomainResourceOperation"
          >
            {{ $t('clusterCenter.domainResources.confirmAction') }}
          </v-btn>
        </v-card-actions>
      </v-card>
    </v-dialog>

    <v-dialog v-model="pingSettingsDialog" max-width="500">
      <v-card>
        <v-card-title>{{ $t('clusterCenter.pingSettings.title') }}</v-card-title>
        <v-card-text>
          <v-switch
            v-model="pingPolicyForm.enabled"
            :label="$t('clusterCenter.pingSettings.autoPing')"
            color="primary"
            hide-details
            class="mb-4"
          />
          <template v-if="pingPolicyForm.enabled">
            <v-select
              v-model="pingPolicyForm.interval"
              :items="PING_INTERVAL_OPTIONS"
              item-title="label"
              item-value="value"
              :label="$t('clusterCenter.pingSettings.interval')"
              class="mb-4"
            />
            <div class="mb-4">
              <div class="text-body-2 mb-2">{{ $t('clusterCenter.pingSettings.probeMethods') }}</div>
              <v-checkbox
                v-for="method in ['icmp', 'tcp', 'http']"
                :key="method"
                v-model="pingPolicyForm.probe_methods"
                :label="method.toUpperCase()"
                :value="method"
                density="compact"
                hide-details
              />
            </div>
            <v-slider
              v-model="pingPolicyForm.alert_threshold"
              :label="$t('clusterCenter.pingSettings.alertThreshold')"
              :min="0"
              :max="1000"
              :step="50"
              thumb-label
              class="mb-4"
            />
            <v-slider
              v-model="pingPolicyForm.max_concurrent"
              :label="$t('clusterCenter.pingSettings.concurrency')"
              :min="1"
              :max="20"
              :step="1"
              thumb-label
            />
          </template>
        </v-card-text>
        <v-card-actions>
          <v-spacer />
          <v-btn @click="pingSettingsDialog = false">{{ $t('cancel') }}</v-btn>
          <v-btn color="primary" :loading="pingSettingsSaving" @click="savePingSettings">{{ $t('save') }}</v-btn>
        </v-card-actions>
      </v-card>
    </v-dialog>

    <v-dialog v-model="showRunTaskDialog" class="app-dialog app-dialog--compact" max-width="460">
      <v-card class="app-card-shell">
        <v-card-title>{{ $t('clusterCenter.processingJobs.dialogTitle') }}</v-card-title>
        <v-card-text class="cluster-center__dialog-body">
          <v-select
            v-model="runTaskForm.taskType"
            :items="availableTaskTypes"
            item-title="label"
            item-value="value"
            :label="$t('clusterCenter.processingJobs.taskTypeLabel')"
          />
          <v-select
            v-model="runTaskForm.scope"
            :items="[
              { label: $t('clusterCenter.processingJobs.scopeDomain'), value: 'domain' },
              { label: $t('clusterCenter.processingJobs.scopeSingle'), value: 'single' },
            ]"
            item-title="label"
            item-value="value"
            :label="$t('clusterCenter.processingJobs.scopeLabel')"
          />
        </v-card-text>
        <v-card-actions>
          <v-spacer />
          <v-btn variant="text" @click="showRunTaskDialog = false">{{ $t('clusterCenter.actions.cancel') }}</v-btn>
          <v-btn color="primary" :loading="runTaskLoading" @click="submitRunTask">
            {{ $t('clusterCenter.processingJobs.startTask') }}
          </v-btn>
        </v-card-actions>
      </v-card>
    </v-dialog>
  </div>
</template>

<script lang="ts" setup>
import { computed, inject, nextTick, onBeforeUnmount, onMounted, ref, watch, type Ref } from 'vue'
import { useRouter } from 'vue-router'
import { push } from 'notivue'

import ClusterDomainActionTree from '@/components/ClusterDomainActionTree.vue'
import DomainResourceInboundEditor from '@/components/DomainResourceInboundEditor.vue'
import DomainResourceUserEditor from '@/components/DomainResourceUserEditor.vue'
import { parseClusterHubJoinUri } from '@/features/clusterHubUri'
import { createDomainInboundResource, createDomainUserResource, deleteDomainInboundResource, deleteDomainUserResource, listDomainResources, retryDomainResourceOperation, updateDomainInboundResource, updateDomainUserResource } from '@/features/domainResourcesApi'
import type { CreateDomainInboundResourcePayload, CreateDomainUserResourcePayload, DomainResourceInboundView, DomainResourceOperationView, DomainResourceUserView } from '@/features/domainResourcesApi'
import { i18n } from '@/locales'
import HttpUtils from '@/plugins/httputil'
import Data from '@/store/modules/data'
import { usePingStore } from '@/store/modules/ping'
import { useClusterStore } from '@/store/modules/cluster'
import type { ClusterDomain, ClusterMember, ClusterOperationStatus, ClusterPanelMemberUpdateResult, ClusterPanelUpdateCheck } from '@/types/clusters'
import type { MeshPairResult, PingPolicy } from '@/types/ping'
import { DEFAULT_PING_POLICY, PING_INTERVAL_OPTIONS } from '@/types/ping'

const router = useRouter()
const globalLoading = inject<Ref<boolean>>('loading', ref(false))

const goToNodeDetail = (member: ClusterMember) => {
  router.push({
    name: 'pages.clusterNodeDetail',
    query: { id: member.nodeId },
  })
}

const pageLoading = ref(true)
const actionLoading = ref(false)
const registerDialog = ref(false)
const confirmDialog = ref(false)
const registerMode = ref<'uri' | 'manual'>('uri')
const domains = ref<ClusterDomain[]>([])
const members = ref<ClusterMember[]>([])
const selectedDomainId = ref<number | null>(null)
const deletingMemberId = ref<number | null>(null)
const leavingDomainId = ref<number | null>(null)
const panelUpdateDialog = ref(false)
const selectedPanelUpdateMember = ref<ClusterMember | null>(null)
const panelUpdatePending = ref<Record<number, boolean>>({})
const panelUpdatePollTimers = new Map<number, number>()
const confirmActionDialog = ref(false)
const pendingAction = ref<'delete' | 'leave' | null>(null)
const pendingActionTarget = ref<ClusterMember | null>(null)
const pendingForceDelete = ref(false)

const form = ref({
  joinUri: '',
  domain: '',
  hubUrlProtocol: 'https',
  hubUrlHost: '',
  token: '',
  displayName: '',
  address: '',
})

const confirmInfo = ref({
  hubUrl: '',
  domain: '',
  token: '',
  baseUrl: '',
  displayName: '',
  address: '',
})

const alreadyExistsDialog = ref(false)
const registerStep = ref(1)
const checkingUrl = ref(false)
const existingDomainData = ref<{ domain: string; hubUrl: string }>({ domain: '', hubUrl: '' })

const selectedDomain = computed(() => domains.value.find((domain) => domain.id === selectedDomainId.value) ?? null)
const selectedDomainMembers = computed(() => members.value.filter((member) => member.domainId === selectedDomainId.value))
const domainInboundTargetMembers = computed(() => {
  if (selectedDomainMembers.value.some((member) => member.isLocal)) return selectedDomainMembers.value
  const localMember = members.value.find((member) => member.isLocal)
  if (!localMember || !selectedDomain.value) return selectedDomainMembers.value
  return [
    { ...localMember, domainId: selectedDomain.value.id },
    ...selectedDomainMembers.value,
  ]
})
const domainUpdateChecks = ref<Record<number, ClusterPanelUpdateCheck>>({})
const domainResourceLoading = ref(false)
const lastDomainResourceOperation = ref<DomainResourceOperationView | null>(null)
const domainInboundDialog = ref(false)
const domainInboundDialogMode = ref<'create' | 'update'>('create')
const editingDomainInboundResource = ref<DomainResourceInboundView | null>(null)
const domainUserDialog = ref(false)
const domainUserDialogMode = ref<'create' | 'update'>('create')
const editingDomainUserResource = ref<DomainResourceUserView | null>(null)
const domainResourceFormError = ref('')
type DomainResourcePendingOperation = {
  kind: 'inbound' | 'user'
  action: 'create' | 'update' | 'delete'
  resourceLabel: string
  resourceId?: string
  payload?: CreateDomainInboundResourcePayload | CreateDomainUserResourcePayload
}
const domainResourceConfirmDialog = ref(false)
const pendingDomainResourceOperation = ref<DomainResourcePendingOperation | null>(null)
const lastDomainInboundGroupIdByDomain = ref<Record<number, string>>({})
const domainInboundGroupIdsByDomain = ref<Record<number, string[]>>({})
const domainInboundResourcesByDomain = ref<Record<number, DomainResourceInboundView[]>>({})
const domainUserResourcesByDomain = ref<Record<number, DomainResourceUserView[]>>({})
const domainOperationInstanceErrors = computed(() =>
  (lastDomainResourceOperation.value?.instances ?? [])
    .filter((instance) => instance.error && instance.error.trim() !== ''),
)
const domainResourceOperationLabelKey = (pending: DomainResourcePendingOperation | null) => {
  if (!pending) return 'clusterCenter.domainResources.confirmAction'
  if (pending.kind === 'inbound' && pending.action === 'create') return 'clusterCenter.domainResources.operationCreateInbound'
  if (pending.kind === 'inbound' && pending.action === 'update') return 'clusterCenter.domainResources.operationUpdateInbound'
  if (pending.kind === 'inbound' && pending.action === 'delete') return 'clusterCenter.domainResources.operationDeleteInbound'
  if (pending.kind === 'user' && pending.action === 'create') return 'clusterCenter.domainResources.operationCreateUser'
  if (pending.kind === 'user' && pending.action === 'update') return 'clusterCenter.domainResources.operationUpdateUser'
  return 'clusterCenter.domainResources.operationDeleteUser'
}
const pendingDomainResourceOperationLabel = computed(() =>
  i18n.global.t(domainResourceOperationLabelKey(pendingDomainResourceOperation.value)).toString(),
)
const mergeDomainUserResources = (users: DomainResourceUserView[]): DomainResourceUserView[] => {
  const userMap = new Map<string, DomainResourceUserView>()
  for (const user of users) {
    const userUuid = user.uuid?.trim()
    if (!userUuid) continue
    const currentNodes = mergeDomainUserAppliedNodes(user.applied_nodes ?? [], [domainUserAppliedNodeFromResource(user)])
    const existing = userMap.get(userUuid)
    if (!existing) {
      userMap.set(userUuid, {
        ...user,
        uuid: userUuid,
        applied_nodes: currentNodes,
      })
      continue
    }
    const preferIncoming = (user.updated_at ?? 0) >= (existing.updated_at ?? 0)
    const preferred = preferIncoming ? user : existing
    userMap.set(userUuid, {
      ...existing,
      ...preferred,
      uuid: userUuid,
      bound_inbound_group_ids: mergeUniqueStrings(existing.bound_inbound_group_ids, user.bound_inbound_group_ids),
      inbounds: mergeUniqueValues(existing.inbounds, user.inbounds),
      applied_nodes: mergeDomainUserAppliedNodes(existing.applied_nodes ?? [], currentNodes),
    })
  }
  return [...userMap.values()]
}
const domainUserAppliedNodeFromResource = (user: DomainResourceUserView) => ({
  client_id: user.client_id,
  node_id: user.node_id || user.nodeId,
  display_name: user.display_name || user.displayName,
  config: user.config,
  sub_token: user.sub_token,
  request_id: user.request_id,
  updated_at: user.updated_at,
})
const mergeDomainUserAppliedNodes = (
  left: NonNullable<DomainResourceUserView['applied_nodes']>,
  right: NonNullable<DomainResourceUserView['applied_nodes']>,
) => {
  const nodes = new Map<string, NonNullable<DomainResourceUserView['applied_nodes']>[number]>()
  for (const node of [...left, ...right]) {
    const key = node.node_id || node.nodeId || node.display_name || node.displayName || String(node.client_id ?? node.request_id ?? '')
    if (!key) continue
    nodes.set(key, node)
  }
  return [...nodes.values()]
}
const mergeUniqueStrings = (left?: string[], right?: string[]) => [
  ...new Set([...(left ?? []), ...(right ?? [])].map((item) => item.trim()).filter(Boolean)),
]
const mergeUniqueValues = (left?: Array<string | number>, right?: Array<string | number>) => [
  ...new Set([...(left ?? []), ...(right ?? [])].map((item) => String(item).trim()).filter(Boolean)),
]
const domainUserAppliedNodeLabel = (user: DomainResourceUserView) => {
  const count = user.applied_nodes?.length ?? 0
  return `${count} ${i18n.global.t('clusterCenter.domainResources.appliedNodes')}`
}
const selectedDomainDefaultInboundGroup = computed(() => {
  if (!selectedDomain.value) return ''
  return lastDomainInboundGroupIdByDomain.value[selectedDomain.value.id] || `domain-${selectedDomain.value.id}`
})
const availableDomainInboundGroups = computed(() => {
  if (!selectedDomain.value) return []
  const domainId = selectedDomain.value.id
  const defaultGroupId = `domain-${domainId}`
  const groupIds = [
    defaultGroupId,
    ...(domainInboundGroupIdsByDomain.value[domainId] ?? []),
  ]
  const lastGroupId = lastDomainInboundGroupIdByDomain.value[domainId]
  if (lastGroupId) groupIds.unshift(lastGroupId)
  return [...new Set(groupIds)]
    .filter(Boolean)
    .map((groupId) => ({ groupId }))
})
const selectedDomainInboundResources = computed(() => {
  if (!selectedDomain.value) return []
  return domainInboundResourcesByDomain.value[selectedDomain.value.id] ?? []
})
const selectedDomainUserResources = computed(() => {
  if (!selectedDomain.value) return []
  return domainUserResourcesByDomain.value[selectedDomain.value.id] ?? []
})

const refreshDomainResourceGroups = async (domainIds: number[]) => {
  const uniqueDomainIds = [...new Set(domainIds.filter((id) => Number.isFinite(id) && id > 0))]
  if (uniqueDomainIds.length === 0) return
  const entries = await Promise.all(uniqueDomainIds.map(async (domainId) => {
    try {
      const resources = await listDomainResources(domainId)
      const inbounds = resources.domain_inbounds
      const users = resources.domain_users
      return [domainId, inbounds, mergeDomainUserResources(users), inbounds.map((inbound) => inbound.group_id).filter(Boolean)] as const
    } catch {
      return [domainId, domainInboundResourcesByDomain.value[domainId] ?? [], domainUserResourcesByDomain.value[domainId] ?? [], domainInboundGroupIdsByDomain.value[domainId] ?? []] as const
    }
  }))
  domainInboundResourcesByDomain.value = {
    ...domainInboundResourcesByDomain.value,
    ...Object.fromEntries(entries.map(([domainId, inbounds]) => [domainId, inbounds])),
  }
  domainUserResourcesByDomain.value = {
    ...domainUserResourcesByDomain.value,
    ...Object.fromEntries(entries.map(([domainId, _inbounds, users]) => [domainId, users])),
  }
  domainInboundGroupIdsByDomain.value = {
    ...domainInboundGroupIdsByDomain.value,
    ...Object.fromEntries(entries.map(([domainId, _inbounds, _users, groupIds]) => [domainId, [...new Set(groupIds)]])),
  }
}

const domainMemberCount = (domainId: number) => members.value.filter((member) => member.domainId === domainId).length
const formatClusterVersionLabel = (version: number) => `${version}`

const openDomainDetail = (domain: ClusterDomain) => {
  selectedDomainId.value = domain.id
  ;(async () => {
    await checkDomainPanelUpdate(domain)
  })()
  pingStore.loadMeshResult(domain.domain).then(result => {
    if (result) meshPingResults.value = result.results
  })
  pingStore.loadPingPolicy(domain.domain).then(p => {
    pingPolicy.value = { ...p }
  })
  clusterStore.fetchScatterTasks(domain.domain)
}

const backToClusterCenter = () => {
  selectedDomainId.value = null
}

const effectiveDomainUpdatePolicy = (policy?: string) => policy === 'manual' ? 'manual' : 'auto'

const formatDomainUpdatePolicy = (policy?: string) => {
  return effectiveDomainUpdatePolicy(policy) === 'manual'
    ? i18n.global.t('clusterCenter.updatePolicies.manual')
    : i18n.global.t('clusterCenter.updatePolicies.auto')
}

const effectiveDomainLatestPanelVersion = (domain: ClusterDomain | null) => {
  if (!domain) return ''
  return domainUpdateChecks.value[domain.id]?.latestVersion || domain.latestPanelVersion || ''
}

const manualUpdateAvailable = (domain: ClusterDomain | null) => {
  if (!domain || effectiveDomainUpdatePolicy(domain.updatePolicy) !== 'manual') return false
  const check = domainUpdateChecks.value[domain.id]
  return Boolean(check?.updateAvailable || domain.panelUpdateAvailable)
}

const formatPanelVersion = (version?: string) => version?.trim() || '-'

const normalizePanelVersion = (version?: string) => {
  const trimmed = version?.trim() ?? ''
  return trimmed.replace(/^[vV]/, '')
}

const comparePanelVersions = (left?: string, right?: string) => {
  const leftNormalized = normalizePanelVersion(left)
  const rightNormalized = normalizePanelVersion(right)
  if (!leftNormalized || !rightNormalized) return 'unknown'
  const leftParts = leftNormalized.split(/[.-]/)
  const rightParts = rightNormalized.split(/[.-]/)
  const maxParts = Math.max(leftParts.length, rightParts.length)
  for (let index = 0; index < maxParts; index += 1) {
    const leftValue = Number.parseInt(leftParts[index] ?? '0', 10)
    const rightValue = Number.parseInt(rightParts[index] ?? '0', 10)
    if (Number.isNaN(leftValue) || Number.isNaN(rightValue)) break
    if (leftValue < rightValue) return 'older'
    if (leftValue > rightValue) return 'newer'
  }
  return 'same'
}

const memberPanelVersionState = (member: ClusterMember) => {
  const latestVersion = effectiveDomainLatestPanelVersion(selectedDomain.value)
  if (!member.panelVersion || !latestVersion) return 'unknown'
  return comparePanelVersions(member.panelVersion, latestVersion) === 'older' ? 'outdated' : 'current'
}

const panelVersionButtonClass = (member: ClusterMember) => {
  const state = memberPanelVersionState(member)
  return {
    'cluster-center__panel-version-button--current': state === 'current',
    'cluster-center__panel-version-button--outdated': state === 'outdated',
    'cluster-center__panel-version-button--unknown': state === 'unknown',
  }
}

const selectedPanelUpdateTargetVersion = computed(() => effectiveDomainLatestPanelVersion(selectedDomain.value))

const memberPanelUpdateDisabled = (member: ClusterMember) => {
  return member.status === 'updating'
    || Boolean(panelUpdatePending.value[member.id])
}

const memberStatusColor = (member: ClusterMember) => member.status === 'updating' ? 'orange' : member.status === 'offline' ? 'red' : 'green'

const memberStatusLabel = (member: ClusterMember) => {
  if (member.status === 'updating') return i18n.global.t('clusterCenter.statuses.updating')
  if (member.status === 'offline') return i18n.global.t('offline')
  return i18n.global.t('online')
}

const openPanelUpdateDialog = (member: ClusterMember) => {
  if (memberPanelUpdateDisabled(member)) return
  selectedPanelUpdateMember.value = member
  panelUpdateDialog.value = true
}

const clearPanelUpdatePolling = (memberId: number) => {
  const timer = panelUpdatePollTimers.get(memberId)
  if (timer) {
    window.clearTimeout(timer)
    panelUpdatePollTimers.delete(memberId)
  }
}

const startPanelUpdateStatusPolling = (memberId: number) => {
  clearPanelUpdatePolling(memberId)
  const poll = async () => {
    await loadData()
    const current = members.value.find((member) => member.id === memberId)
    if (!current || current.status !== 'updating') {
      panelUpdatePending.value = { ...panelUpdatePending.value, [memberId]: false }
      clearPanelUpdatePolling(memberId)
      return
    }
    const timer = window.setTimeout(poll, 5000)
    panelUpdatePollTimers.set(memberId, timer)
  }
  const timer = window.setTimeout(poll, 5000)
  panelUpdatePollTimers.set(memberId, timer)
}

const memberDisplayNameDrafts = ref<Record<number, string>>({})

const onMemberDisplayNameInput = (member: ClusterMember, value: string) => {
  memberDisplayNameDrafts.value = { ...memberDisplayNameDrafts.value, [member.id]: value }
}

const commitMemberDisplayName = async (member: ClusterMember) => {
  const draft = memberDisplayNameDrafts.value[member.id]
  if (draft === undefined) return
  const trimmed = draft.trim()
  const current = (member.displayName || member.name || '').trim()
  if (trimmed === current) {
    delete memberDisplayNameDrafts.value[member.id]
    memberDisplayNameDrafts.value = { ...memberDisplayNameDrafts.value }
    return
  }
  const msg = await HttpUtils.put(`api/cluster/members/${member.id}/display-name`, { displayName: trimmed })
  if (msg.success) {
    members.value = members.value.map((item) => item.id === member.id
      ? { ...item, displayName: trimmed }
      : item)
    push.success({ title: i18n.global.t('success'), message: i18n.global.t('clusterCenter.table.displayNameSaved') })
  }
  delete memberDisplayNameDrafts.value[member.id]
  memberDisplayNameDrafts.value = { ...memberDisplayNameDrafts.value }
}

const memberCountryDisplay = (member: ClusterMember): string => {
  const code = (member.countryCode || '').trim()
  if (!code) return ''
  const flag = code.toUpperCase().replace(/./g, (c) => String.fromCodePoint(127397 + c.charCodeAt(0)))
  let name = ''
  try {
    const intl = new (Intl as any).DisplayNames([i18n.global.locale?.toString() || 'en'], { type: 'region' })
    const resolved = intl.of(code.toUpperCase())
    name = typeof resolved === 'string' && resolved ? resolved : ''
  } catch {
    name = ''
  }
  return `${flag} ${name || code.toUpperCase()}`.trim()
}


const confirmPanelUpdate = async () => {
  const member = selectedPanelUpdateMember.value
  if (!member) return
  const targetVersion = selectedPanelUpdateTargetVersion.value
  panelUpdatePending.value = { ...panelUpdatePending.value, [member.id]: true }
  const msg = await HttpUtils.post(`api/cluster/members/${member.id}/panel-update`, {
    targetVersion,
  })
  if (msg.success) {
    const result = msg.obj as ClusterPanelMemberUpdateResult
    members.value = members.value.map((item) => item.id === member.id
      ? { ...item, status: result.status || 'updating' }
      : item)
    panelUpdateDialog.value = false
    startPanelUpdateStatusPolling(member.id)
    push.success({
      title: i18n.global.t('success'),
      message: i18n.global.t('clusterCenter.panelUpdate.requested'),
      duration: 5000,
    })
  } else {
    panelUpdatePending.value = { ...panelUpdatePending.value, [member.id]: false }
  }
}

const checkDomainPanelUpdate = async (domain: ClusterDomain) => {
  const msg = await HttpUtils.post(`api/cluster/domains/${domain.id}/update-check`, {})
  if (!msg.success || !msg.obj) return null
  const result = msg.obj as ClusterPanelUpdateCheck
  domainUpdateChecks.value = {
    ...domainUpdateChecks.value,
    [domain.id]: result,
  }
  domains.value = domains.value.map((item) => item.id === domain.id
    ? {
        ...item,
        updatePolicy: result.updatePolicy,
        latestPanelVersion: result.latestVersion || item.latestPanelVersion,
        panelUpdateAvailable: result.updateAvailable,
      }
    : item)
  return result
}

const openDomainInboundResourceDialog = () => {
  if (!selectedDomain.value) return
  domainResourceFormError.value = ''
  domainInboundDialogMode.value = 'create'
  editingDomainInboundResource.value = null
  domainInboundDialog.value = true
}

const openDomainInboundEditDialog = (inbound: DomainResourceInboundView) => {
  if (!selectedDomain.value) return
  domainResourceFormError.value = ''
  domainInboundDialogMode.value = 'update'
  editingDomainInboundResource.value = inbound
  domainInboundDialog.value = true
}

const openDomainUserResourceDialog = () => {
  if (!selectedDomain.value) return
  domainResourceFormError.value = ''
  domainUserDialogMode.value = 'create'
  editingDomainUserResource.value = null
  domainUserDialog.value = true
}

const openDomainUserEditDialog = (user: DomainResourceUserView) => {
  if (!selectedDomain.value) return
  domainResourceFormError.value = ''
  domainUserDialogMode.value = 'update'
  editingDomainUserResource.value = user
  domainUserDialog.value = true
}

const openDomainResourceConfirmDialog = (pending: DomainResourcePendingOperation) => {
  if (!selectedDomain.value) return
  pendingDomainResourceOperation.value = pending
  domainResourceConfirmDialog.value = true
}

const cancelDomainResourceOperation = () => {
  if (domainResourceLoading.value) return
  domainResourceConfirmDialog.value = false
  pendingDomainResourceOperation.value = null
}

const submitDomainInboundResource = async (payload: CreateDomainInboundResourcePayload) => {
  if (!selectedDomain.value) return
  domainResourceFormError.value = ''
  openDomainResourceConfirmDialog({
    kind: 'inbound',
    action: domainInboundDialogMode.value,
    resourceLabel: payload.group_id,
    resourceId: domainInboundDialogMode.value === 'update'
      ? editingDomainInboundResource.value?.group_id?.trim() || payload.group_id
      : payload.group_id,
    payload: { ...payload },
  })
}

const executeDomainInboundResource = async (
  payload: CreateDomainInboundResourcePayload,
  action = domainInboundDialogMode.value,
  resourceId = editingDomainInboundResource.value?.group_id?.trim() || payload.group_id,
) => {
  if (!selectedDomain.value) return
  domainResourceLoading.value = true
  try {
    lastDomainInboundGroupIdByDomain.value = {
      ...lastDomainInboundGroupIdByDomain.value,
      [selectedDomain.value.id]: payload.group_id,
    }
    domainInboundGroupIdsByDomain.value = {
      ...domainInboundGroupIdsByDomain.value,
      [selectedDomain.value.id]: [
        ...new Set([...(domainInboundGroupIdsByDomain.value[selectedDomain.value.id] ?? []), payload.group_id]),
      ],
    }
    if (action === 'update') {
      const editingGroupId = resourceId
      lastDomainResourceOperation.value = await updateDomainInboundResource(selectedDomain.value.id, editingGroupId, {
        ...payload,
      })
    } else {
      lastDomainResourceOperation.value = await createDomainInboundResource(selectedDomain.value.id, {
        ...payload,
      })
    }
    await refreshDomainResourceGroups([selectedDomain.value.id])
    domainInboundDialog.value = false
    domainInboundDialogMode.value = 'create'
    editingDomainInboundResource.value = null
  } catch (error: any) {
    domainResourceFormError.value = error?.message ?? String(error)
    push.error({ title: i18n.global.t('failed'), message: error?.message ?? String(error) })
    throw error
  } finally {
    domainResourceLoading.value = false
  }
}

const requestDeleteDomainInboundGroup = (inbound: DomainResourceInboundView) => {
  if (!selectedDomain.value || !inbound.group_id.trim()) return
  domainResourceFormError.value = ''
  openDomainResourceConfirmDialog({
    kind: 'inbound',
    action: 'delete',
    resourceLabel: inbound.group_id,
    resourceId: inbound.group_id,
  })
}

const deleteDomainInboundGroup = async (groupId: string) => {
  if (!selectedDomain.value || !groupId.trim()) return
  domainResourceLoading.value = true
  try {
    lastDomainResourceOperation.value = await deleteDomainInboundResource(selectedDomain.value.id, groupId)
    const remainingGroups = (domainInboundGroupIdsByDomain.value[selectedDomain.value.id] ?? [])
      .filter((current) => current !== groupId)
    domainInboundGroupIdsByDomain.value = {
      ...domainInboundGroupIdsByDomain.value,
      [selectedDomain.value.id]: remainingGroups,
    }
    domainInboundResourcesByDomain.value = {
      ...domainInboundResourcesByDomain.value,
      [selectedDomain.value.id]: (domainInboundResourcesByDomain.value[selectedDomain.value.id] ?? [])
        .filter((inbound) => inbound.group_id !== groupId),
    }
    if (lastDomainInboundGroupIdByDomain.value[selectedDomain.value.id] === groupId) {
      lastDomainInboundGroupIdByDomain.value = {
        ...lastDomainInboundGroupIdByDomain.value,
        [selectedDomain.value.id]: remainingGroups[0] || `domain-${selectedDomain.value.id}`,
      }
    }
    await refreshDomainResourceGroups([selectedDomain.value.id])
  } catch (error: any) {
    domainResourceFormError.value = error?.message ?? String(error)
    push.error({ title: i18n.global.t('failed'), message: error?.message ?? String(error) })
    throw error
  } finally {
    domainResourceLoading.value = false
  }
}

const submitDomainUserResource = async (payload: CreateDomainUserResourcePayload) => {
  if (!selectedDomain.value) return
  domainResourceFormError.value = ''
  openDomainResourceConfirmDialog({
    kind: 'user',
    action: domainUserDialogMode.value,
    resourceLabel: payload.user.name?.trim() || payload.user.uuid?.trim() || '-',
    resourceId: domainUserDialogMode.value === 'update'
      ? editingDomainUserResource.value?.uuid?.trim() || payload.user.uuid?.trim() || ''
      : payload.user.uuid?.trim() || '',
    payload: { ...payload },
  })
}

const executeDomainUserResource = async (
  payload: CreateDomainUserResourcePayload,
  action = domainUserDialogMode.value,
  resourceId = editingDomainUserResource.value?.uuid?.trim() || payload.user.uuid?.trim() || '',
) => {
  if (!selectedDomain.value) return
  domainResourceLoading.value = true
  try {
    if (action === 'update') {
      const editingUserUuid = resourceId
      lastDomainResourceOperation.value = await updateDomainUserResource(selectedDomain.value.id, editingUserUuid, {
        ...payload,
      })
    } else {
      lastDomainResourceOperation.value = await createDomainUserResource(selectedDomain.value.id, {
        ...payload,
      })
    }
    await refreshDomainResourceGroups([selectedDomain.value.id])
    domainUserDialog.value = false
    domainUserDialogMode.value = 'create'
    editingDomainUserResource.value = null
  } catch (error: any) {
    domainResourceFormError.value = error?.message ?? String(error)
    push.error({ title: i18n.global.t('failed'), message: error?.message ?? String(error) })
    throw error
  } finally {
    domainResourceLoading.value = false
  }
}

const requestDeleteDomainUser = (user: DomainResourceUserView) => {
  if (!selectedDomain.value || !user.uuid.trim()) return
  domainResourceFormError.value = ''
  openDomainResourceConfirmDialog({
    kind: 'user',
    action: 'delete',
    resourceLabel: user.name?.trim() || user.uuid,
    resourceId: user.uuid,
  })
}

const deleteDomainUser = async (userUUID: string) => {
  if (!selectedDomain.value || !userUUID.trim()) return
  domainResourceLoading.value = true
  try {
    lastDomainResourceOperation.value = await deleteDomainUserResource(selectedDomain.value.id, userUUID)
    domainUserResourcesByDomain.value = {
      ...domainUserResourcesByDomain.value,
      [selectedDomain.value.id]: (domainUserResourcesByDomain.value[selectedDomain.value.id] ?? [])
        .filter((user) => user.uuid !== userUUID),
    }
    await refreshDomainResourceGroups([selectedDomain.value.id])
  } catch (error: any) {
    domainResourceFormError.value = error?.message ?? String(error)
    push.error({ title: i18n.global.t('failed'), message: error?.message ?? String(error) })
    throw error
  } finally {
    domainResourceLoading.value = false
  }
}

const executePendingDomainResourceOperation = async (pending: DomainResourcePendingOperation) => {
  if (pending.kind === 'inbound' && pending.action === 'delete') {
    await deleteDomainInboundGroup(pending.resourceId ?? pending.resourceLabel)
    return
  }
  if (pending.kind === 'inbound') {
    await executeDomainInboundResource(
      pending.payload as CreateDomainInboundResourcePayload,
      pending.action === 'update' ? 'update' : 'create',
      pending.resourceId,
    )
    return
  }
  if (pending.kind === 'user' && pending.action === 'delete') {
    await deleteDomainUser(pending.resourceId ?? pending.resourceLabel)
    return
  }
  await executeDomainUserResource(
    pending.payload as CreateDomainUserResourcePayload,
    pending.action === 'update' ? 'update' : 'create',
    pending.resourceId,
  )
}

const confirmDomainResourceOperation = async () => {
  const pending = pendingDomainResourceOperation.value
  if (!pending) return
  try {
    await executePendingDomainResourceOperation(pending)
    domainResourceConfirmDialog.value = false
    pendingDomainResourceOperation.value = null
  } catch {
    // Operation helpers already surface form/toast errors.
  }
}

const retryLastDomainResourceOperation = async () => {
  if (!lastDomainResourceOperation.value?.operationId) return
  domainResourceLoading.value = true
  try {
    lastDomainResourceOperation.value = await retryDomainResourceOperation(lastDomainResourceOperation.value.operationId)
  } catch (error: any) {
    push.error({ title: i18n.global.t('failed'), message: error?.message ?? String(error) })
  } finally {
    domainResourceLoading.value = false
  }
}

const domainOperationStatusColor = (status: string) => {
  switch (status) {
    case 'applied': return 'green'
    case 'failed':
    case 'timeout': return 'red'
    case 'partial': return 'orange'
    case 'dispatching': return 'blue'
    default: return 'grey'
  }
}

const formatDomainResourceOperationStatus = (status: string) => {
  const translated = i18n.global.t(`clusterCenter.domainResources.operationStatuses.${status}`)
  return translated === `clusterCenter.domainResources.operationStatuses.${status}` ? status : translated.toString()
}

const isUsableAbsoluteUrl = (value: string) => {
  try {
    new URL(value)
    return true
  } catch {
    return false
  }
}

const resolvePanelBaseUrl = () => {
  const rawBaseUrl = String((window as any).BASE_URL ?? '/')
  const normalizedBaseUrl = rawBaseUrl.endsWith('/') ? rawBaseUrl : `${rawBaseUrl}/`

  try {
    return new URL(normalizedBaseUrl, window.location.origin).toString()
  } catch {
    return ''
  }
}

const resolveNodeAddress = () => {
  try {
    if (Data().subURI) {
      return new URL(Data().subURI).hostname.toLowerCase()
    }
  } catch {
    // Fall back to the panel hostname when the subscription URI is incomplete.
  }
  return window.location.hostname.replace(/^\[|\]$/g, '').toLowerCase()
}

const normalizeClusterBaseUrl = (value: string) => {
  const trimmed = value.trim()
  if (!trimmed) return ''

  try {
    const url = new URL(trimmed)
    url.protocol = url.protocol.toLowerCase()
    url.hostname = url.hostname.toLowerCase()
    url.hash = ''
    url.search = ''
    url.pathname = url.pathname.replace(/\/+$/, '')
    if ((url.protocol === 'https:' && url.port === '443') || (url.protocol === 'http:' && url.port === '80')) {
      url.port = ''
    }
    return url.toString().replace(/\/+$/, '')
  } catch {
    return trimmed.toLowerCase().replace(/\/+$/, '')
  }
}

const deriveDisplayNameFromBaseUrl = (baseUrl: string) => {
  const match = baseUrl.trim().match(/^https?:\/\/([^/:?#]+)(?::\d+)?(?:[/?#]|$)/i)
  return match?.[1]?.toLowerCase() ?? ''
}

const validateAndCheckDomain = async () => {
  if (registerMode.value === 'uri') {
    const uri = form.value.joinUri.trim()
    const parsed = parseClusterHubJoinUri(uri)
    if (!parsed) {
      push.error({ title: i18n.global.t('failed'), message: i18n.global.t('clusterCenter.validation.invalidJoinUri') })
      return
    }
    const panelBaseUrl = resolvePanelBaseUrl()
    confirmInfo.value = {
      hubUrl: `${parsed.protocol}://${parsed.host}`,
      domain: parsed.domainId,
      token: parsed.token,
      baseUrl: panelBaseUrl,
      displayName: deriveDisplayNameFromBaseUrl(panelBaseUrl),
      address: resolveNodeAddress(),
    }
  } else {
    const domain = form.value.domain.trim()
    const hubUrlHost = form.value.hubUrlHost.trim()
    const hubUrl = `${form.value.hubUrlProtocol}://${hubUrlHost}`

    if (!domain || !hubUrlHost || !form.value.token) {
      push.error({ title: i18n.global.t('failed'), message: i18n.global.t('clusterCenter.validation.required') })
      return
    }
    if (!isUsableAbsoluteUrl(hubUrl)) {
      push.error({ title: i18n.global.t('failed'), message: i18n.global.t('clusterCenter.validation.hubUrl') })
      return
    }
    if (!isUsableAbsoluteUrl(resolvePanelBaseUrl())) {
      push.error({ title: i18n.global.t('failed'), message: i18n.global.t('clusterCenter.validation.panelUrl') })
      return
    }

    const panelBaseUrl = resolvePanelBaseUrl()
    confirmInfo.value = {
      hubUrl,
      domain,
      token: form.value.token,
      baseUrl: panelBaseUrl,
      displayName: deriveDisplayNameFromBaseUrl(panelBaseUrl),
      address: resolveNodeAddress(),
    }
  }

  form.value.displayName = confirmInfo.value.displayName
  form.value.address = confirmInfo.value.address

  await checkPanelUrlExists()
}

const checkPanelUrlExists = async () => {
  checkingUrl.value = true
  const panelBaseUrl = resolvePanelBaseUrl()
  const normalizedPanelBaseUrl = normalizeClusterBaseUrl(panelBaseUrl)

  try {
    const snapshotUrl = `${confirmInfo.value.hubUrl}/v1/domains/${encodeURIComponent(confirmInfo.value.domain)}/snapshot`
    const resp = await fetch(snapshotUrl, {
      headers: { 'X-Domain-Token': confirmInfo.value.token },
    })
    if (resp.ok) {
      const snapshot = await resp.json()
      const members = snapshot.members || []
      const existingMember = members.find(
        (m: any) => normalizeClusterBaseUrl(m.base_url || m.baseUrl || '') === normalizedPanelBaseUrl,
      )
      if (existingMember) {
        existingDomainData.value = {
          domain: confirmInfo.value.domain,
          hubUrl: confirmInfo.value.hubUrl,
        }
        alreadyExistsDialog.value = true
        checkingUrl.value = false
        return
      }
    }
  } catch {
    // If we can't reach the hub or domain doesn't exist, proceed with registration
  }

  checkingUrl.value = false
  registerStep.value = 2
}

const showConfirmDialog = () => {
  const displayName = form.value.displayName.trim()
  if (!displayName) {
    push.error({ title: i18n.global.t('failed'), message: i18n.global.t('clusterCenter.validation.displayName') })
    return
  }
  confirmInfo.value.displayName = displayName
  const address = form.value.address.trim()
  if (!address) {
    push.error({ title: i18n.global.t('failed'), message: i18n.global.t('clusterCenter.validation.nodeAddress') })
    return
  }
  confirmInfo.value.address = address
  confirmDialog.value = true
}

const onRegisterDialogClose = () => {
  registerStep.value = 1
  form.value = { joinUri: '', domain: '', hubUrlProtocol: 'https', hubUrlHost: '', token: '', displayName: '', address: '' }
}

const pullExistingDomain = async () => {
  alreadyExistsDialog.value = false
  registerDialog.value = false
  registerStep.value = 1
  form.value = { joinUri: '', domain: '', hubUrlProtocol: 'https', hubUrlHost: '', token: '', displayName: '', address: '' }
  actionLoading.value = true

  const panelBaseUrl = resolvePanelBaseUrl()
  const nodeAddress = confirmInfo.value.address || resolveNodeAddress()
  const displayName = deriveDisplayNameFromBaseUrl(panelBaseUrl)
  const registerMsg = await HttpUtils.post('api/cluster/register', {
    domain: existingDomainData.value.domain,
    hubUrl: existingDomainData.value.hubUrl,
    token: confirmInfo.value.token,
    baseUrl: panelBaseUrl,
    address: nodeAddress,
    name: '',
    displayName,
  })

  if (registerMsg.success) {
    const operation = registerMsg.obj as ClusterOperationStatus
    if (operation?.id) {
      await pollOperation(operation.id)
    }
    await loadData()
    push.success({
      title: i18n.global.t('success'),
      message: i18n.global.t('clusterCenter.successRegistered'),
      duration: 5000,
    })
  }

  actionLoading.value = false
}

const confirmAndSubmit = async () => {
  const panelBaseUrl = resolvePanelBaseUrl()
  if (!isUsableAbsoluteUrl(panelBaseUrl)) {
    push.error({ title: i18n.global.t('failed'), message: i18n.global.t('clusterCenter.validation.panelUrl') })
    return
  }

  actionLoading.value = true
  registerDialog.value = false
  registerStep.value = 1

  try {
    const registerMsg = await HttpUtils.post('api/cluster/register', {
      domain: confirmInfo.value.domain,
      hubUrl: confirmInfo.value.hubUrl,
      token: confirmInfo.value.token,
      baseUrl: panelBaseUrl,
      address: confirmInfo.value.address,
      name: '',
      displayName: confirmInfo.value.displayName,
    })

    if (registerMsg.success) {
      const operation = registerMsg.obj as ClusterOperationStatus
      if (operation?.id) {
        await pollOperation(operation.id)
      }
      await loadData()
      confirmDialog.value = false
      form.value = { joinUri: '', domain: '', hubUrlProtocol: 'https', hubUrlHost: '', token: '', displayName: '', address: '' }
      push.success({
        title: i18n.global.t('success'),
        message: i18n.global.t('clusterCenter.successRegistered'),
        duration: 5000,
      })
    }
  } finally {
    actionLoading.value = false
  }
}

const loadData = async () => {
  pageLoading.value = true
  try {
    const [domainsMsg, membersMsg] = await Promise.all([
      HttpUtils.get('api/cluster/domains'),
      HttpUtils.get('api/cluster/members'),
    ])
    if (domainsMsg.success) {
      domains.value = Array.isArray(domainsMsg.obj) ? domainsMsg.obj : []
      if (selectedDomainId.value && !domains.value.some((domain) => domain.id === selectedDomainId.value)) {
        selectedDomainId.value = null
      }
    }
    if (membersMsg.success) {
      members.value = Array.isArray(membersMsg.obj) ? membersMsg.obj : []
      for (const member of members.value) {
        if (member.status !== 'updating' && panelUpdatePending.value[member.id]) {
          panelUpdatePending.value = { ...panelUpdatePending.value, [member.id]: false }
          clearPanelUpdatePolling(member.id)
        }
      }
    }
    await refreshDomainResourceGroups(domains.value.map((domain) => domain.id))
  } finally {
    pageLoading.value = false
  }
}

const pollOperation = async (operationId: string) => {
  let current: ClusterOperationStatus | null = null
  for (const delay of [0, 300, 700, 1500, 3000]) {
    if (delay > 0) {
      await new Promise((resolve) => setTimeout(resolve, delay))
    }
    const operationMsg = await HttpUtils.get(`api/cluster/operations/${operationId}`)
    if (!operationMsg.success) {
      return null
    }
    current = operationMsg.obj as ClusterOperationStatus
    if (!current || current.state === 'completed') {
      return current
    }
  }
  return current
}

const syncClusterState = async () => {
  const msg = await HttpUtils.post('api/cluster/sync', {})
  const operation = msg.obj as ClusterOperationStatus | null
  if (operation?.message) {
    push.error({ title: i18n.global.t('failed'), message: operation.message })
  }
  await loadData()
  return msg
}

const manualSync = async () => {
  actionLoading.value = true
  try {
    await syncClusterState()
  } finally {
    actionLoading.value = false
  }
}

const requestDeleteMember = (member: ClusterMember, force = false) => {
  pendingAction.value = 'delete'
  pendingActionTarget.value = member
  pendingForceDelete.value = force
  confirmActionDialog.value = true
}

const domainInboundStatusColor = (status?: string) => {
  if (status === 'active' || status === 'applied') return 'green'
  return domainOperationStatusColor(status || '')
}

const requestLeaveDomain = (force = false) => {
  pendingAction.value = 'leave'
  pendingActionTarget.value = selectedDomainMembers.value.find(m => m.isLocal) ?? null
  pendingForceDelete.value = force
  confirmActionDialog.value = true
}

const confirmAction = async () => {
  confirmActionDialog.value = false
  actionLoading.value = true
  let queuedForceConfirm = false
  if (pendingAction.value === 'delete' && pendingActionTarget.value) {
    queuedForceConfirm = await deleteMember(pendingActionTarget.value, pendingForceDelete.value)
  } else if (pendingAction.value === 'leave') {
    queuedForceConfirm = await leaveDomain(selectedDomain.value, pendingForceDelete.value)
  }
  actionLoading.value = false
  if (!queuedForceConfirm) {
    resetPendingAction()
  }
}

const resetPendingAction = () => {
  pendingAction.value = null
  pendingActionTarget.value = null
  pendingForceDelete.value = false
}

const cancelPendingAction = () => {
  confirmActionDialog.value = false
  resetPendingAction()
}

const shouldOfferForceDelete = (msg: any) => {
  const message = String(msg?.msg ?? '').toLowerCase()
  return message.includes('cleanup') ||
    message.includes('domain_cleanup_failed') ||
    message.includes('delete cluster member') ||
    message.includes('leave cluster domain')
}

const deleteMember = async (member: ClusterMember, force = false) => {
  deletingMemberId.value = member.id
  const msg = await HttpUtils.delete(`api/cluster/members/${member.id}${force ? '?force=1' : ''}`)
  if (msg.success) {
    await loadData()
  } else if (!force && shouldOfferForceDelete(msg)) {
    requestDeleteMember(member, true)
    deletingMemberId.value = null
    return true
  }
  deletingMemberId.value = null
  return false
}

const leaveDomain = async (domain: ClusterDomain | null, force = false) => {
  if (!domain) return false

  leavingDomainId.value = domain.id
  const msg = await HttpUtils.delete(`api/cluster/domains/${domain.id}${force ? '?force=1' : ''}`)
  if (msg.success) {
    selectedDomainId.value = null
    await loadData()
  } else if (!force && shouldOfferForceDelete(msg)) {
    requestLeaveDomain(true)
    leavingDomainId.value = null
    return true
  }
  leavingDomainId.value = null
  return false
}

onMounted(async () => {
  pageLoading.value = true
  globalLoading.value = true
  try {
    await syncClusterState()
  } finally {
    pageLoading.value = false
    globalLoading.value = false
  }
})

onBeforeUnmount(() => {
  for (const memberId of panelUpdatePollTimers.keys()) {
    clearPanelUpdatePolling(memberId)
  }
  stopClusterLogPoll()
  clusterStore.stopPolling()
  globalLoading.value = false
})

// --- Cluster logs ---
interface ClusterLogEntry {
  time: string
  level: string
  direction: string
  action: string
  fields: Record<string, unknown>
}

const clusterLogs = ref<ClusterLogEntry[]>([])
const logContainer = ref<HTMLElement | null>(null)
let clusterLogTimer: ReturnType<typeof setInterval> | null = null
const userScrolledAwayFromLatest = ref(false)

function onLogContainerScroll() {
  const el = logContainer.value
  if (!el) return
  const atBottom = el.scrollHeight - el.scrollTop - el.clientHeight < 5
  userScrolledAwayFromLatest.value = !atBottom
}

function formatLogFields(fields: Record<string, unknown>): string {
  if (!fields) return ''
  const parts = Object.entries(fields)
    .filter(([k]) => k !== 'domain')
    .map(([k, v]) => {
      if (Array.isArray(v)) return `${k}=[${v.join(',')}]`
      return `${k}=${v}`
    })
  return parts.join(' ')
}

async function loadClusterLogs() {
  if (!selectedDomainId.value) return
  const msg = await HttpUtils.get('api/cluster/logs', { domain_id: selectedDomainId.value, count: 200 })
  if (msg.success && Array.isArray(msg.obj)) {
    clusterLogs.value = [...msg.obj].reverse()
    await nextTick()
    scrollLogToBottom()
  }
}

function scrollLogToBottom() {
  if (userScrolledAwayFromLatest.value) return
  const el = logContainer.value
  if (el) el.scrollTop = el.scrollHeight
}

function startClusterLogPoll() {
  stopClusterLogPoll()
  loadClusterLogs()
  clusterLogTimer = setInterval(loadClusterLogs, 5000)
}

function stopClusterLogPoll() {
  if (clusterLogTimer) {
    clearInterval(clusterLogTimer)
    clusterLogTimer = null
  }
}

watch(selectedDomainId, (id) => {
  clusterLogs.value = []
  userScrolledAwayFromLatest.value = false
  if (id) {
    startClusterLogPoll()
    clusterStore.startPolling(selectedDomain.value?.domain ?? '')
  } else {
    stopClusterLogPoll()
    clusterStore.stopPolling()
  }
})

const pingStore = usePingStore()
const clusterStore = useClusterStore()
const showRunTaskDialog = ref(false)
const runTaskLoading = ref(false)
const runTaskForm = ref({
  taskType: 'mesh.latency',
  scope: 'domain',
})

const availableTaskTypes = [
  { label: 'Mesh Latency', value: 'mesh.latency' },
]

const goToTaskResult = (taskId: string) => {
  if (!selectedDomainId.value) return
  router.push({
    name: 'pages.clusterScatterTaskResult',
    params: { domainId: String(selectedDomainId.value), taskId },
  })
}

const taskStatusColor = (status: string) => {
  switch (status) {
    case 'completed': return 'green'
    case 'failed': return 'red'
    case 'timeout': return 'orange'
    case 'dispatching':
    case 'collecting':
    case 'aggregating': return 'blue'
    default: return 'grey'
  }
}

const submitRunTask = async () => {
  if (!selectedDomain.value) return
  runTaskLoading.value = true
  try {
    const result = await clusterStore.createScatterTask(
      selectedDomain.value.domain,
      runTaskForm.value.taskType,
      runTaskForm.value.scope,
      {},
    )
    if (result) {
      showRunTaskDialog.value = false
      await clusterStore.fetchScatterTasks(selectedDomain.value.domain)
      push.success({
        title: i18n.global.t('success'),
        message: i18n.global.t('clusterCenter.processingJobs.taskStarted'),
        duration: 3000,
      })
    } else {
      push.error({
        title: i18n.global.t('failed'),
        message: i18n.global.t('clusterCenter.processingJobs.taskFailed'),
      })
    }
  } finally {
    runTaskLoading.value = false
  }
}

const meshPingLoading = ref(false)
const meshPingResults = ref<MeshPairResult[]>([])

function memberLatency(nodeId: string): string {
  const results = meshPingResults.value.filter(r => r.target_member_id === nodeId && r.success)
  if (results.length === 0) {
    const any = meshPingResults.value.filter(r => r.target_member_id === nodeId)
    if (any.length > 0) return 'ERROR'
    return '-'
  }
  const avg = results.reduce((s, r) => s + (r.latency_ms ?? 0), 0) / results.length
  return `${avg.toFixed(0)}ms`
}

function memberLatencyStyle(nodeId: string): Record<string, string> {
  const results = meshPingResults.value.filter(r =>
    r.target_member_id === nodeId && (r.success || !r.success)
  )
  if (results.length === 0) return { color: 'var(--app-text-3)' }

  const allFailed = results.every(r => !r.success)
  if (allFailed) return { color: '#721c24', fontWeight: 'bold' }

  const successResults = results.filter(r => r.success)
  if (successResults.length === 0) return { color: 'var(--app-text-3)' }

  const avg = successResults.reduce((s, r) => s + (r.latency_ms ?? 0), 0) / successResults.length
  if (avg < 50) return { color: '#155724', fontWeight: '600' }
  if (avg < 150) return { color: '#856404', fontWeight: '600' }
  if (avg < 300) return { color: '#b45309', fontWeight: '600' }
  return { color: '#721c24', fontWeight: '600' }
}

function upsertMeshPairResult(results: MeshPairResult[], result: MeshPairResult): MeshPairResult[] {
  const index = results.findIndex(r =>
    r.source_member_id === result.source_member_id && r.target_member_id === result.target_member_id
  )
  if (index === -1) return [...results, result]
  const next = [...results]
  next[index] = result
  return next
}

async function pingAllDomainMembers() {
  if (!selectedDomain.value) return
  meshPingLoading.value = true
  meshPingResults.value = []
  try {
    const result = await pingStore.triggerMeshPingStream(selectedDomain.value.domain, result => {
      meshPingResults.value = upsertMeshPairResult(meshPingResults.value, result)
    })
    meshPingResults.value = result.results
  } catch {
    // error handled by store
  } finally {
    try {
      await syncClusterState()
    } catch {
      // refresh errors are surfaced by the shared HTTP layer
    }
    meshPingLoading.value = false
  }
}

const pingSettingsDialog = ref(false)
const pingSettingsSaving = ref(false)
const pingPolicy = ref<PingPolicy>({ ...DEFAULT_PING_POLICY })
const pingPolicyForm = ref<PingPolicy>({ ...DEFAULT_PING_POLICY })

async function openPingSettingsDialog() {
  if (!selectedDomain.value) return
  const policy = await pingStore.loadPingPolicy(selectedDomain.value.domain)
  pingPolicy.value = { ...policy }
  pingPolicyForm.value = { ...policy }
  pingSettingsDialog.value = true
}

async function savePingSettings() {
  if (!selectedDomain.value) return
  pingSettingsSaving.value = true
  try {
    await pingStore.savePingPolicy(selectedDomain.value.domain, pingPolicyForm.value)
    pingPolicy.value = { ...pingPolicyForm.value }
    pingSettingsDialog.value = false
    push.success({
      title: i18n.global.t('success'),
      message: i18n.global.t('clusterCenter.pingSettings.saved'),
      duration: 3000,
    })
  } catch {
    push.error({
      title: i18n.global.t('failed'),
      message: i18n.global.t('clusterCenter.pingSettings.saveFailed'),
    })
  } finally {
    pingSettingsSaving.value = false
  }
}

function formatAutoPingTime(): string {
  const interval = pingPolicy.value.interval
  if (interval < 60) return `${interval}s`
  if (interval % 60 === 0) return `${interval / 60}min`
  return `${interval}s`
}
</script>

<style scoped>
.cluster-center__hero {
  min-height: max-content;
  overflow: visible;
}

.cluster-center__actions {
  display: flex;
  flex-wrap: wrap;
  gap: 12px;
}

.cluster-center__refresh-btn {
  backdrop-filter: blur(var(--app-blur-panel));
  background: linear-gradient(
    180deg,
    color-mix(in srgb, var(--app-surface-3) 94%, transparent),
    color-mix(in srgb, var(--app-surface-2) 98%, transparent)
  ) !important;
  border: 1px solid color-mix(in srgb, var(--app-text-2) 24%, var(--app-border-2)) !important;
  box-shadow: var(--app-shadow-button) !important;
  color: var(--app-text-1) !important;
}

.cluster-center__refresh-btn:hover {
  background: linear-gradient(
    180deg,
    color-mix(in srgb, var(--app-state-info) 10%, var(--app-surface-3)),
    color-mix(in srgb, var(--app-state-info) 6%, var(--app-surface-2))
  ) !important;
  border-color: color-mix(in srgb, var(--app-state-info) 42%, var(--app-border-2)) !important;
}

.cluster-center__refresh-btn:focus-visible {
  outline: none;
  box-shadow: var(--app-shadow-button), 0 0 0 4px color-mix(in srgb, var(--app-state-info) 18%, transparent) !important;
}

.cluster-center__refresh-btn :deep(.v-btn__overlay) {
  opacity: 0.04;
}

.cluster-center__grid {
  align-items: stretch;
}

.cluster-center__card-title {
  align-items: baseline;
  display: flex;
  flex-wrap: wrap;
  gap: 10px 14px;
  justify-content: space-between;
  width: 100%;
}

.cluster-center__domain-prompt {
  color: var(--app-text-3);
  font-size: 13px;
  font-weight: 500;
  line-height: 1.5;
}

.cluster-center__detail {
  display: grid;
  gap: 16px;
}

.cluster-center__detail-actions {
  align-items: center;
  display: flex;
  flex-wrap: wrap;
  gap: 12px;
  justify-content: flex-start;
}

.cluster-center__logs {
  height: 100%;
}

.cluster-center__log-count {
  background: color-mix(in srgb, var(--app-state-info) 16%, transparent);
  border-radius: 999px;
  color: var(--app-state-info);
  font-size: 12px;
  font-weight: 700;
  line-height: 1;
  padding: 4px 8px;
}

.cluster-center__log-container {
  background: color-mix(in srgb, var(--app-surface-1) 90%, transparent);
  border: 1px solid var(--app-border-1);
  border-radius: 14px;
  font-family: var(--app-font-mono, ui-monospace, monospace);
  font-size: 12px;
  line-height: 1.7;
  max-height: 260px;
  overflow-y: auto;
  padding: 12px 14px;
}

.cluster-center__log-line {
  align-items: baseline;
  display: flex;
  flex-wrap: wrap;
  gap: 8px;
  padding: 2px 0;
}

.cluster-center__log-time {
  color: var(--app-text-3);
  flex-shrink: 0;
  font-size: 11px;
}

.cluster-center__log-dir {
  border-radius: 4px;
  flex-shrink: 0;
  font-size: 10px;
  font-weight: 700;
  letter-spacing: 0.06em;
  padding: 1px 5px;
}

.cluster-center__log-line--info .cluster-center__log-dir {
  background: color-mix(in srgb, #55b3ff 14%, transparent);
  color: #55b3ff;
}

.cluster-center__log-line--error .cluster-center__log-dir {
  background: color-mix(in srgb, #e55353 14%, transparent);
  color: #e55353;
}

.cluster-center__log-line--warn .cluster-center__log-dir {
  background: color-mix(in srgb, #f5b51b 14%, transparent);
  color: #f5b51b;
}

.cluster-center__log-line--debug .cluster-center__log-dir {
  background: color-mix(in srgb, var(--app-text-3) 10%, transparent);
  color: var(--app-text-3);
}

.cluster-center__log-action {
  color: var(--app-text-1);
  font-weight: 600;
}

.cluster-center__log-fields {
  color: var(--app-text-3);
  overflow: hidden;
  text-overflow: ellipsis;
  white-space: nowrap;
}

.cluster-center__domains,
.cluster-center__members {
  height: 100%;
}

.cluster-center__empty {
  color: var(--app-text-3);
  padding: 20px 0;
}

.cluster-center__domain-list {
  display: grid;
  gap: 12px;
}

.cluster-center__domain-card {
  background: color-mix(in srgb, var(--app-surface-2) 86%, transparent);
  border: 1px solid var(--app-border-1);
  border-radius: 20px;
  color: inherit;
  cursor: pointer;
  display: grid;
  gap: 8px;
  padding: 16px;
  text-align: left;
  transition: border-color var(--app-motion-fast) var(--app-ease-standard), transform var(--app-motion-fast) var(--app-ease-standard);
}

.cluster-center__domain-card:hover,
.cluster-center__domain-card--active {
  border-color: color-mix(in srgb, var(--app-state-info) 36%, var(--app-border-2));
  transform: translateY(-1px);
}

.cluster-center__domain-head {
  align-items: center;
  display: flex;
  gap: 12px;
  justify-content: space-between;
}

.cluster-center__version,
.cluster-center__domain-card .cluster-center__domain-meta,
.cluster-center__domain-url {
  color: var(--app-text-3);
  font-size: 13px;
}

.cluster-center__selected-head {
  align-items: center;
  display: flex;
  flex-wrap: wrap;
  gap: 10px;
  min-width: 0;
}

.cluster-center__selected-version {
  border: 1px solid var(--app-border-1);
  border-radius: 999px;
  line-height: 1;
  padding: 6px 9px;
}

.cluster-center__detail-panel {
  display: grid;
  gap: 14px;
  grid-template-columns: minmax(0, 1fr) 280px;
}

.cluster-center__domain-meta {
  display: grid;
  gap: 8px;
}

.cluster-center__meta-row {
  align-items: start;
  border-bottom: 1px solid var(--app-border-1);
  display: grid;
  gap: 10px;
  grid-template-columns: 112px minmax(0, 1fr);
  padding-bottom: 8px;
}

.cluster-center__meta-row:last-child {
  border-bottom: none;
  padding-bottom: 0;
}

.cluster-center__meta-label {
  color: var(--app-text-3);
  font-size: 12px;
  letter-spacing: 0.04em;
}

.cluster-center__meta-label--header {
  display: inline-flex;
  margin-bottom: 10px;
  text-transform: uppercase;
}

.cluster-center__meta-value {
  font-size: 14px;
  overflow-wrap: anywhere;
}

.cluster-center__policy-badges {
  align-items: center;
  display: flex;
  flex-wrap: wrap;
  gap: 8px;
}

.cluster-center__policy-badge,
.cluster-center__update-available-badge,
.cluster-center__panel-version-button {
  align-items: center;
  border: 1px solid var(--app-border-1);
  border-radius: 999px;
  display: inline-flex;
  font-family: var(--app-font-mono, ui-monospace, monospace);
  font-size: 12px;
  font-weight: 700;
  line-height: 1;
  min-height: 24px;
  padding: 5px 8px;
  white-space: nowrap;
}

.cluster-center__panel-version-button {
  text-transform: none;
}

.cluster-center__policy-badge {
  color: var(--app-state-info);
}

.cluster-center__update-available-badge,
.cluster-center__panel-version-button--outdated {
  background: color-mix(in srgb, #f5b51b 13%, transparent);
  border-color: color-mix(in srgb, #f5b51b 48%, var(--app-border-1));
  color: #8a5a00;
}

.cluster-center__panel-version-button--current {
  background: color-mix(in srgb, #1a9f62 13%, transparent);
  border-color: color-mix(in srgb, #1a9f62 44%, var(--app-border-1));
  color: #0f7044;
}

.cluster-center__panel-version-button--unknown {
  color: var(--app-text-3);
}

.cluster-center__actions-tree {
  background: color-mix(in srgb, var(--app-surface-2) 82%, transparent);
  border: 1px solid var(--app-border-1);
  border-radius: 16px;
  min-width: 0;
  padding: 14px 16px;
}

.cluster-center__member-table-wrap {
  border: 1px solid var(--app-border-1);
  border-radius: 18px;
  overflow: hidden;
}

.cluster-center__member-table {
  border-collapse: collapse;
  width: 100%;
}

.cluster-center__member-table th,
.cluster-center__member-table td {
  border-bottom: 1px solid var(--app-border-1);
  padding: 14px 16px;
  text-align: left;
}

.cluster-center__member-table tbody tr:last-child td {
  border-bottom: none;
}

.cluster-center__member-table th {
  color: var(--app-text-3);
  font-size: 12px;
  letter-spacing: 0.12em;
  text-transform: uppercase;
}

.cluster-center__member-node {
  align-items: center;
  display: flex;
  flex-wrap: wrap;
  gap: 8px;
}

.cluster-center__local-badge {
  border: 1px solid color-mix(in srgb, var(--app-state-info) 40%, var(--app-border-1));
  border-radius: 999px;
  color: var(--app-state-info);
  font-size: 12px;
  font-weight: 700;
  line-height: 1;
  padding: 5px 8px;
}

.cluster-center__dialog-body {
  display: grid;
  gap: 12px;
}

.cluster-center__register-tabs {
  margin: 0 16px;
}

.cluster-center__register-tabs :deep(.v-tab) {
  font-size: 14px;
  letter-spacing: 0.04em;
  text-transform: none;
}

.cluster-center__hub-url-field {
  align-items: center;
  background: color-mix(in srgb, var(--app-surface-2) 86%, transparent);
  border: 1px solid var(--app-border-1);
  border-radius: 8px;
  display: flex;
  gap: 0;
  padding: 0 12px;
  transition: border-color var(--app-motion-fast) var(--app-ease-standard);
}

.cluster-center__hub-url-field:focus-within {
  border-color: var(--app-state-info);
  box-shadow: 0 0 0 3px color-mix(in srgb, var(--app-state-info) 15%, transparent);
}

.cluster-center__hub-url-protocol {
  flex: 0 0 72px;
  max-width: 72px;
  min-width: 72px;
  width: 72px;
}

.cluster-center__hub-url-protocol :deep(.v-field) {
  min-height: unset;
  padding: 0;
}

.cluster-center__hub-url-protocol :deep(.v-field__input) {
  align-items: center;
  display: flex;
  font-size: 14px;
  line-height: 1;
  min-height: unset;
  padding: 6px 2px 6px 0;
}

.cluster-center__hub-url-protocol :deep(.v-field__append-inner) {
  padding-inline-start: 0;
}

.cluster-center__hub-url-protocol :deep(.v-field__outline),
.cluster-center__hub-url-protocol :deep(.v-field__overlay) {
  display: none;
}

.cluster-center__hub-url-protocol :deep(.v-select__selection) {
  text-overflow: clip;
}

.cluster-center__hub-url-sep {
  color: var(--app-text-3);
  flex-shrink: 0;
  font-size: 14px;
  line-height: 1;
  margin-right: 4px;
  pointer-events: none;
  user-select: none;
}

.cluster-center__hub-url-host {
  flex: 1 1 auto;
  min-width: 0;
}

.cluster-center__hub-url-host :deep(.v-field) {
  min-height: unset;
  padding: 0;
}

.cluster-center__hub-url-host :deep(.v-field__input) {
  align-items: center;
  display: flex;
  min-height: unset;
  padding: 6px 0;
}

.cluster-center__hub-url-host :deep(.v-field__outline) {
  display: none;
}

.cluster-center__confirm-title {
  line-height: 1.3;
  padding: 24px 24px 10px;
}

.cluster-center__confirm-body {
  padding-top: 8px;
}

.cluster-center__confirm-table-wrap {
  border: 1px solid var(--app-border-1);
  border-radius: 18px;
  overflow: hidden;
}

.cluster-center__confirm-table {
  border-collapse: collapse;
  width: 100%;
}

.cluster-center__confirm-table tr {
  border-bottom: 1px solid var(--app-border-1);
}

.cluster-center__confirm-table tbody tr:last-child {
  border-bottom: none;
}

.cluster-center__confirm-table td {
  padding: 14px 16px;
}

.cluster-center__confirm-label {
  color: var(--app-text-3);
  font-size: 13px;
  white-space: nowrap;
  width: 1px;
}

.cluster-center__confirm-value {
  font-size: 14px;
  word-break: break-all;
}

.cluster-center__confirm-token {
  font-family: var(--app-font-mono, ui-monospace, monospace);
  letter-spacing: 0.06em;
}

.cluster-center__panel-update-copy {
  color: var(--app-text-2);
  font-size: 14px;
  line-height: 1.5;
  margin: 0;
}

@media (max-width: 960px) {
  .cluster-center__actions {
    flex-direction: column;
  }

  .cluster-center__detail-panel {
    grid-template-columns: 1fr;
  }

  .cluster-center__member-table,
  .cluster-center__member-table thead,
  .cluster-center__member-table tbody,
  .cluster-center__member-table tr,
  .cluster-center__member-table th,
  .cluster-center__member-table td {
    display: block;
  }

  .cluster-center__member-table thead {
    display: none;
  }

  .cluster-center__member-table tr {
    border-bottom: 1px solid var(--app-border-1);
  }

  .cluster-center__member-table tbody tr:last-child {
    border-bottom: none;
  }
}

.cluster-center__step-indicator {
  align-items: center;
  background: color-mix(in srgb, var(--app-surface-2) 86%, transparent);
  border: 1px solid var(--app-border-1);
  border-radius: 12px;
  display: flex;
  gap: 10px;
  padding: 12px 14px;
}

.cluster-center__step-label {
  color: var(--app-text-3);
  font-size: 12px;
  font-weight: 600;
  letter-spacing: 0.08em;
  text-transform: uppercase;
}

.cluster-center__step-value {
  font-size: 14px;
  font-weight: 600;
  word-break: break-all;
}

@media (max-width: 640px) {
  .cluster-center__meta-row {
    gap: 6px;
    grid-template-columns: 1fr;
  }
}

.cluster-center__auto-ping-indicator {
  display: inline-flex;
  align-items: center;
  gap: 6px;
  font-size: 12px;
  color: var(--app-text-2);
}

.cluster-center__auto-ping-dot {
  width: 6px;
  height: 6px;
  border-radius: 50%;
  background: #28a745;
  display: inline-block;
}

.cluster-center__auto-ping-text {
  font-size: 11px;
}

.cluster-center__task-type {
  font-family: var(--app-font-mono, ui-monospace, monospace);
  font-size: 12px;
  font-weight: 600;
}

.cluster-center__jobs {
  height: 100%;
}

.cluster-center__domain-resource-actions {
  display: flex;
  flex-wrap: wrap;
  gap: 10px;
}

.cluster-center__domain-resources {
  overflow: hidden;
}

.cluster-center__domain-resource-section {
  display: grid;
  gap: 10px;
  margin-top: 16px;
}

.cluster-center__domain-resource-section-title {
  color: var(--app-text-2);
  font-size: 12px;
  font-weight: 800;
  letter-spacing: 0.08em;
  text-transform: uppercase;
}

.cluster-center__domain-inbound-list {
  display: grid;
  gap: 10px;
}

.cluster-center__domain-inbound-row {
  align-items: center;
  background: color-mix(in srgb, var(--app-surface-2) 82%, transparent);
  border: 1px solid var(--app-border-1);
  border-radius: 8px;
  display: flex;
  gap: 12px;
  justify-content: space-between;
  min-width: 0;
  padding: 10px 12px;
}

.cluster-center__domain-inbound-main {
  display: grid;
  gap: 3px;
  min-width: 0;
}

.cluster-center__domain-inbound-group {
  font-family: var(--app-font-mono, ui-monospace, monospace);
  font-size: 13px;
  font-weight: 700;
  overflow-wrap: anywhere;
}

.cluster-center__domain-inbound-type {
  color: var(--app-text-3);
  font-size: 12px;
}

.cluster-center__domain-resource-node-count {
  color: var(--app-text-3);
  font-size: 12px;
}

.cluster-center__domain-inbound-actions {
  align-items: center;
  display: flex;
  flex: 0 0 auto;
  gap: 4px;
}

.cluster-center__operation-status {
  align-items: center;
  background: color-mix(in srgb, var(--app-surface-2) 82%, transparent);
  border: 1px solid var(--app-border-1);
  border-radius: 12px;
  display: flex;
  flex-wrap: wrap;
  gap: 10px;
  margin-top: 14px;
  padding: 10px 12px;
}

.cluster-center__operation-status-label {
  color: var(--app-text-2);
  font-size: 12px;
  font-weight: 700;
}

.cluster-center__operation-errors {
  background: color-mix(in srgb, #e55353 8%, var(--app-surface-2));
  border: 1px solid color-mix(in srgb, #e55353 32%, var(--app-border-1));
  border-radius: 12px;
  display: grid;
  gap: 8px;
  margin-top: 10px;
  padding: 12px;
}

.cluster-center__operation-errors-title {
  color: #e55353;
  font-size: 12px;
  font-weight: 700;
  text-transform: uppercase;
}

.cluster-center__operation-error {
  display: grid;
  gap: 4px;
}

.cluster-center__operation-error-node {
  color: var(--app-text-2);
  font-family: var(--app-font-mono, ui-monospace, monospace);
  font-size: 12px;
  font-weight: 700;
}

.cluster-center__operation-error-message {
  color: var(--app-text-1);
  font-size: 13px;
  overflow-wrap: anywhere;
}

.cluster-center__domain-resource-dialog :deep(.v-card-title) {
  padding-bottom: 8px;
}

.cluster-center__domain-resource-dialog {
  max-height: min(760px, calc(100vh - 48px));
}

.cluster-center__resource-dialog-body {
  gap: 14px;
}

.cluster-center__resource-form-grid {
  display: grid;
  gap: 12px;
  grid-template-columns: minmax(0, 1fr) minmax(0, 1fr);
}

.cluster-center__resource-form-grid--primary {
  grid-template-columns: minmax(0, 1.2fr) minmax(180px, 0.8fr);
}

.cluster-center__resource-switch-row {
  align-items: center;
  display: flex;
  flex-wrap: wrap;
  gap: 8px 22px;
}

.cluster-center__field-note {
  color: var(--app-text-3);
  font-size: 12px;
  line-height: 1.5;
  margin-top: -4px;
}

.cluster-center__json-input :deep(textarea) {
  font-family: var(--app-font-mono, ui-monospace, monospace);
  font-size: 12px;
  line-height: 1.6;
}

.cluster-center__json-input--compact :deep(textarea) {
  min-height: 84px;
}

@media (max-width: 640px) {
  .cluster-center__resource-form-grid,
  .cluster-center__resource-form-grid--primary {
    grid-template-columns: 1fr;
  }
}
</style>
