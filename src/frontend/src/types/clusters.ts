export interface ClusterDomain {
  id: number
  domain: string
  hubUrl: string
  lastVersion: number
}

export interface ClusterMember {
  id: number
  domainId: number
  nodeId: string
  name: string
  baseUrl: string
  lastVersion: number
}

export interface ClusterOperationStatus {
  id: string
  state: string
  message?: string
}
