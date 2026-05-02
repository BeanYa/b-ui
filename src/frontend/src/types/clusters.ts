export interface ClusterDomain {
  id: number
  domain: string
  hubUrl: string
  communicationEndpointPath: string
  communicationProtocolVersion: string
  lastVersion: number
  updatePolicy: 'auto' | 'manual'
  latestPanelVersion?: string
  panelUpdateAvailable?: boolean
  supportedActions: string[]
}

export interface ClusterMember {
  id: number
  domainId: number
  nodeId: string
  name: string
  displayName: string
  baseUrl: string
  lastVersion: number
  isLocal: boolean
  panelVersion: string
  status: string
}

export interface ClusterMemberConnection {
  nodeId: string
  name: string
  displayName: string
  baseUrl: string
  token: string
}

export interface ClusterOperationStatus {
  id: string
  state: string
  message?: string
}

export interface ClusterPanelUpdateCheck {
  currentVersion: string
  latestVersion?: string
  comparison: string
  updateAvailable: boolean
  updatePolicy: 'auto' | 'manual'
  autoUpdate: boolean
  updateStarted: boolean
}

export interface ClusterPanelMemberUpdateResult {
  nodeId: string
  currentVersion: string
  targetVersion?: string
  status: string
  updateStarted: boolean
}

export interface ScatterTaskSummary {
  taskId: string
  taskType: string
  status: string
  scope: string
  progress: string
  createdAt: string
  completedAt?: string
}

export interface ScatterTaskResultDetail {
  taskId: string
  taskType: string
  status: string
  result: any
  generatedAt?: string
}
