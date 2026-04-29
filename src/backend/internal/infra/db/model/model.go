package model

import "encoding/json"

type Setting struct {
	Id    uint   `json:"id" form:"id" gorm:"primaryKey;autoIncrement"`
	Key   string `json:"key" form:"key"`
	Value string `json:"value" form:"value"`
}

type Tls struct {
	Id     uint            `json:"id" form:"id" gorm:"primaryKey;autoIncrement"`
	Name   string          `json:"name" form:"name"`
	Server json.RawMessage `json:"server" form:"server"`
	Client json.RawMessage `json:"client" form:"client"`
}

type User struct {
	Id         uint   `json:"id" form:"id" gorm:"primaryKey;autoIncrement"`
	Username   string `json:"username" form:"username"`
	Password   string `json:"password" form:"password"`
	LastLogins string `json:"lastLogin"`
}

type Client struct {
	Id       uint            `json:"id" form:"id" gorm:"primaryKey;autoIncrement"`
	Enable   bool            `json:"enable" form:"enable"`
	Name     string          `json:"name" form:"name"`
	Config   json.RawMessage `json:"config,omitempty" form:"config"`
	Inbounds json.RawMessage `json:"inbounds" form:"inbounds"`
	Links    json.RawMessage `json:"links,omitempty" form:"links"`
	Volume   int64           `json:"volume" form:"volume"`
	Expiry   int64           `json:"expiry" form:"expiry"`
	Down     int64           `json:"down" form:"down"`
	Up       int64           `json:"up" form:"up"`
	Desc     string          `json:"desc" form:"desc"`
	Group    string          `json:"group" form:"group"`

	// Delay start and periodic reset
	DelayStart bool  `json:"delayStart" form:"delayStart" gorm:"default:false;not null"`
	AutoReset  bool  `json:"autoReset" form:"autoReset" gorm:"default:false;not null"`
	ResetDays  int   `json:"resetDays" form:"resetDays" gorm:"default:0;not null"`
	NextReset  int64 `json:"nextReset" form:"nextReset" gorm:"default:0;not null"`
	TotalUp    int64 `json:"totalUp" form:"totalUp" gorm:"default:0;not null"`
	TotalDown  int64 `json:"totalDown" form:"totalDown" gorm:"default:0;not null"`
}

type Stats struct {
	Id        uint64 `json:"id" gorm:"primaryKey;autoIncrement"`
	DateTime  int64  `json:"dateTime"`
	Resource  string `json:"resource"`
	Tag       string `json:"tag"`
	Direction bool   `json:"direction"`
	Traffic   int64  `json:"traffic"`
}

type Changes struct {
	Id       uint64          `json:"id" gorm:"primaryKey;autoIncrement"`
	DateTime int64           `json:"dateTime"`
	Actor    string          `json:"actor"`
	Key      string          `json:"key"`
	Action   string          `json:"action"`
	Obj      json.RawMessage `json:"obj"`
}

type Tokens struct {
	Id     uint   `json:"id" form:"id" gorm:"primaryKey;autoIncrement"`
	Desc   string `json:"desc" form:"desc"`
	Token  string `json:"token" form:"token"`
	Expiry int64  `json:"expiry" form:"expiry"`
	UserId uint   `json:"userId" form:"userId"`
	User   *User  `json:"user" gorm:"foreignKey:UserId;references:Id"`
}

type ClusterLocalNode struct {
	Id         uint   `json:"id" gorm:"primaryKey;autoIncrement"`
	NodeID     string `json:"nodeId" gorm:"uniqueIndex"`
	PublicKey  string `json:"publicKey"`
	PrivateKey string `json:"privateKey"`
}

type ClusterDomain struct {
	Id                           uint   `json:"id" gorm:"primaryKey;autoIncrement"`
	Domain                       string `json:"domain" gorm:"uniqueIndex"`
	HubURL                       string `json:"hubUrl"`
	TokenEncrypted               string `json:"-"`
	CommunicationEndpointPath    string `json:"communicationEndpointPath" gorm:"default:/_cluster"`
	CommunicationProtocolVersion string `json:"communicationProtocolVersion" gorm:"default:v1"`
	LastVersion                  int64  `json:"lastVersion" gorm:"default:0"`
	UpdatePolicy                 string `json:"updatePolicy" gorm:"default:auto"`
	LatestPanelVersion           string `json:"latestPanelVersion"`
	PanelUpdateAvailable         bool   `json:"panelUpdateAvailable" gorm:"default:false"`
}

type ClusterMember struct {
	Id                 uint           `json:"id" gorm:"primaryKey;autoIncrement"`
	NodeID             string         `json:"nodeId" gorm:"uniqueIndex:idx_cluster_domain_node"`
	Name               string         `json:"name"`
	DisplayName        string         `json:"displayName"`
	PanelVersion       string         `json:"panelVersion"`
	Status             string         `json:"status" gorm:"default:online"`
	BaseURL            string         `json:"baseUrl"`
	PublicKey          string         `json:"publicKey"`
	PeerTokenEncrypted string         `json:"-"`
	DomainID           uint           `json:"domainId" gorm:"uniqueIndex:idx_cluster_domain_node"`
	LastVersion        int64          `json:"lastVersion" gorm:"default:0"`
	LastNotifiedAt     int64          `json:"lastNotifiedAt" gorm:"default:0"`
	LastNotifiedValue  int64          `json:"lastNotifiedValue" gorm:"default:0"`
	Domain             *ClusterDomain `json:"domain,omitempty" gorm:"foreignKey:DomainID;references:Id"`
}

type ClusterPeerReachability struct {
	Id                    uint   `json:"id" gorm:"primaryKey;autoIncrement"`
	DomainID              uint   `json:"domainId" gorm:"uniqueIndex:idx_cluster_reachability_domain_target"`
	TargetNodeID          string `json:"targetNodeId" gorm:"uniqueIndex:idx_cluster_reachability_domain_target"`
	State                 string `json:"state" gorm:"default:unknown"`
	LastObservedAt        int64  `json:"lastObservedAt" gorm:"default:0"`
	LastSuccessAt         int64  `json:"lastSuccessAt" gorm:"default:0"`
	LastFailureAt         int64  `json:"lastFailureAt" gorm:"default:0"`
	ConsecutiveFailures   int64  `json:"consecutiveFailures" gorm:"default:0"`
	NextProbeAt           int64  `json:"nextProbeAt" gorm:"default:0"`
	LastObservationSource string `json:"lastObservationSource" gorm:"default:''"`
}

type ClusterPeerEventLog struct {
	Id          uint   `json:"id" gorm:"primaryKey;autoIncrement"`
	MessageID   string `json:"messageId" gorm:"uniqueIndex"`
	DomainID    string `json:"domainId" gorm:"index"`
	Direction   string `json:"direction"`
	SourceNode  string `json:"sourceNode" gorm:"index"`
	Action      string `json:"action" gorm:"index"`
	Envelope    string `json:"envelope"`
	PayloadHash string `json:"payloadHash"`
	Signature   string `json:"signature"`
	CreatedAt   int64  `json:"createdAt" gorm:"index"`
}

type ClusterPeerEventState struct {
	Id             uint   `json:"id" gorm:"primaryKey;autoIncrement"`
	MessageID      string `json:"messageId" gorm:"uniqueIndex"`
	IdempotencyKey string `json:"idempotencyKey" gorm:"index;uniqueIndex:idx_cluster_peer_domain_idempotency,where:idempotency_key <> ''"`
	SourceNode     string `json:"sourceNode" gorm:"index;uniqueIndex:idx_cluster_peer_domain_source_seq,where:source_seq > 0 AND source_node <> ''"`
	SourceSeq      int64  `json:"sourceSeq" gorm:"index;uniqueIndex:idx_cluster_peer_domain_source_seq,where:source_seq > 0 AND source_node <> ''"`
	DomainID       string `json:"domainId" gorm:"index;uniqueIndex:idx_cluster_peer_domain_idempotency,where:idempotency_key <> '';uniqueIndex:idx_cluster_peer_domain_source_seq,where:source_seq > 0 AND source_node <> ''"`
	Action         string `json:"action" gorm:"index"`
	PayloadHash    string `json:"payloadHash"`
	Status         string `json:"status" gorm:"index"`
	Error          string `json:"error"`
	CreatedAt      int64  `json:"createdAt"`
	UpdatedAt      int64  `json:"updatedAt"`
}

type ClusterPeerAckState struct {
	Id          uint   `json:"id" gorm:"primaryKey;autoIncrement"`
	MessageID   string `json:"messageId" gorm:"index;uniqueIndex:idx_cluster_peer_ack_target"`
	TargetNode  string `json:"targetNode" gorm:"index;uniqueIndex:idx_cluster_peer_ack_target"`
	Status      string `json:"status" gorm:"index"`
	Attempts    int    `json:"attempts"`
	NextRetryAt int64  `json:"nextRetryAt" gorm:"index"`
	Error       string `json:"error"`
	UpdatedAt   int64  `json:"updatedAt"`
}

type ClusterPeerWorkflowState struct {
	Id         uint   `json:"id" gorm:"primaryKey;autoIncrement"`
	WorkflowID string `json:"workflowId" gorm:"index;uniqueIndex:idx_cluster_peer_workflow_step"`
	StepID     string `json:"stepId" gorm:"index;uniqueIndex:idx_cluster_peer_workflow_step"`
	DomainID   string `json:"domainId" gorm:"index"`
	NodeID     string `json:"nodeId" gorm:"index"`
	Status     string `json:"status" gorm:"index"`
	ResultHash string `json:"resultHash"`
	Error      string `json:"error"`
	CreatedAt  int64  `json:"createdAt"`
	UpdatedAt  int64  `json:"updatedAt"`
}

type ClusterPeerSchedule struct {
	Id          uint   `json:"id" gorm:"primaryKey;autoIncrement"`
	ScheduleID  string `json:"scheduleId" gorm:"uniqueIndex"`
	DomainID    string `json:"domainId" gorm:"index"`
	OwnerNodeID string `json:"ownerNodeId" gorm:"index"`
	Action      string `json:"action" gorm:"index"`
	RouteJSON   string `json:"routeJson"`
	PayloadJSON string `json:"payloadJson"`
	NextRunAt   int64  `json:"nextRunAt" gorm:"index"`
	LastRunAt   int64  `json:"lastRunAt"`
	RunCount    int    `json:"runCount"`
	MaxRuns     int    `json:"maxRuns"`
	Enabled     bool   `json:"enabled" gorm:"index"`
}
