export interface ClusterDomain {
  id: number
  domain: string
  hubUrl: string
  communicationEndpointPath: string
  communicationProtocolVersion: string
  lastVersion: number
  supportedActions: string[]
}

export interface ClusterMember {
  id: number
  domainId: number
  nodeId: string
  name: string
  baseUrl: string
  lastVersion: number
  isLocal: boolean
}

export interface ClusterOperationStatus {
  id: string
  state: string
  message?: string
}
