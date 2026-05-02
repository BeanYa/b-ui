package service

import (
	"context"
	"encoding/json"
	"sync"
	"time"

	"github.com/BeanYa/b-ui/src/backend/internal/infra/db/model"
	"gorm.io/gorm"
)

// TaskStatus represents the lifecycle state of a scatter-gather task.
type TaskStatus string

const (
	TaskStatusCreated     TaskStatus = "created"
	TaskStatusDispatching TaskStatus = "dispatching"
	TaskStatusCollecting  TaskStatus = "collecting"
	TaskStatusAggregating TaskStatus = "aggregating"
	TaskStatusCompleted   TaskStatus = "completed"
	TaskStatusFailed      TaskStatus = "failed"
	TaskStatusTimeout     TaskStatus = "timeout"
)

// ScatterGatherTask represents a single scatter-gather operation.
type ScatterGatherTask struct {
	TaskID        string
	DomainID      string
	TaskType      string
	Scope         string
	Status        TaskStatus
	Params        map[string]any
	CorrelationID string
	TimeoutMs     int64
	CreatedAt     time.Time
	Results       map[string]NodeResult
	TotalNodes    int
	ResultChan    chan NodeResult
	Ctx           context.Context
	Cancel        context.CancelFunc
}

// DomainTaskManager manages an independent queue and worker pool for one domain.
type DomainTaskManager struct {
	domainID       string
	queue          chan *ScatterGatherTask
	workers        int
	active         map[string]*ScatterGatherTask
	mu             sync.RWMutex
	db             *gorm.DB
	delivery       *ClusterPeerDeliveryService
	identity       ClusterLocalIdentityService
	syncStore      clusterSyncStore
	secretProvider clusterSecretProvider
	hubClient      clusterHubClient
	hubURL         string
}

// Enqueue adds a task to the domain's queue and persists it to storage.
func (m *DomainTaskManager) Enqueue(task *ScatterGatherTask) {
	task.Status = TaskStatusCreated
	InsertScatterTaskWithRolling(m.db, &model.ClusterScatterTask{
		TaskID:     task.TaskID,
		DomainID:   task.DomainID,
		TaskType:   task.TaskType,
		Scope:      task.Scope,
		Status:     string(task.Status),
		ParamsJSON: mustMarshalJSON(task.Params),
		CreatedAt:  task.CreatedAt,
	})
	m.mu.Lock()
	m.active[task.TaskID] = task
	m.mu.Unlock()
	m.queue <- task
}

func (m *DomainTaskManager) worker() {
	for task := range m.queue {
		m.executeTask(task)
	}
}

func (m *DomainTaskManager) executeTask(task *ScatterGatherTask) {
	// 1. Dispatching phase
	task.Status = TaskStatusDispatching
	UpdateScatterTaskStatus(m.db, task.TaskID, string(task.Status))

	handler, ok := GetScatterHandler(task.TaskType)
	if !ok {
		task.Status = TaskStatusFailed
		UpdateScatterTaskStatus(m.db, task.TaskID, string(task.Status))
		m.mu.Lock()
		delete(m.active, task.TaskID)
		m.mu.Unlock()
		return
	}

	// Execute local work
	localResult, err := handler.ExecuteLocal(task.Ctx, task.DomainID, task.Params)
	local, localErr := m.identity.GetOrCreate()
	if localErr == nil && local != nil {
		if err != nil {
			task.Results[local.NodeID] = NodeResult{
				NodeID: local.NodeID,
				Status: "failed",
				Error:  err.Error(),
			}
		} else {
			task.Results[local.NodeID] = NodeResult{
				NodeID: local.NodeID,
				Status: "completed",
				Result: localResult,
			}
		}
	}

	// Broadcast task.scatter to all other domain members
	members, memberErr := m.resolveDomainMembers()
	if memberErr == nil {
		task.TotalNodes = len(members)
		for _, member := range members {
			if local != nil && member.NodeID == local.NodeID {
				continue
			}
			m.sendScatterBroadcast(task, &member)
		}
	}

	// 2. Collecting phase
	task.Status = TaskStatusCollecting
	UpdateScatterTaskStatus(m.db, task.TaskID, string(task.Status))

	m.collectResults(task, handler)
}

// resolveDomainMembers looks up the domain by name and returns its members.
func (m *DomainTaskManager) resolveDomainMembers() ([]model.ClusterMember, error) {
	domains, err := m.syncStore.ListDomains()
	if err != nil {
		return nil, err
	}
	for _, d := range domains {
		if d.Domain == m.domainID {
			return m.syncStore.GetMembers(d.Id)
		}
	}
	return nil, nil
}

func (m *DomainTaskManager) collectResults(task *ScatterGatherTask, handler ScatterGatherHandler) {
	timeout := time.After(time.Duration(task.TimeoutMs) * time.Millisecond)
	retried := false

	for {
		select {
		case result := <-task.ResultChan:
			task.Results[result.NodeID] = result
			if len(task.Results) >= task.TotalNodes {
				goto aggregate
			}
		case <-timeout:
			if !retried {
				m.retryUnresponsive(task)
				retried = true
				timeout = time.After(30 * time.Second)
				continue
			}
			m.markUnresponsiveTimeout(task)
			goto aggregate
		case <-task.Ctx.Done():
			m.markUnresponsiveTimeout(task)
			goto aggregate
		}
	}

aggregate:
	task.Status = TaskStatusAggregating
	UpdateScatterTaskStatus(m.db, task.TaskID, string(task.Status))

	results := make([]NodeResult, 0, len(task.Results))
	for _, r := range task.Results {
		results = append(results, r)
	}

	aggregated, err := handler.Aggregate(task.Ctx, results)
	if err != nil {
		task.Status = TaskStatusFailed
		UpdateScatterTaskStatus(m.db, task.TaskID, string(task.Status))
		m.mu.Lock()
		delete(m.active, task.TaskID)
		m.mu.Unlock()
		return
	}

	task.Status = TaskStatusCompleted
	UpdateScatterTaskStatus(m.db, task.TaskID, string(task.Status))

	reportJSON := mustMarshalJSON(aggregated)
	SaveScatterResult(m.db, &model.ClusterScatterResult{
		TaskID:      task.TaskID,
		ReportJSON:  reportJSON,
		GeneratedAt: time.Now(),
	})

	// Report to Hub if scope is domain
	if task.Scope == "domain" && m.hubClient != nil && m.hubURL != "" {
		// Hub reporting will be wired later when ReportDomainReport is added to the interface
	}

	m.mu.Lock()
	delete(m.active, task.TaskID)
	m.mu.Unlock()
}

func (m *DomainTaskManager) sendScatterBroadcast(task *ScatterGatherTask, member *model.ClusterMember) {
	local, err := m.identity.GetOrCreate()
	if err != nil {
		return
	}

	payload := map[string]interface{}{
		"task_type":  task.TaskType,
		"task_id":    task.TaskID,
		"scope":      task.Scope,
		"timeout_ms": task.TimeoutMs,
		"params":     task.Params,
	}

	msg, err := NewClusterPeerMessage(
		task.DomainID,
		0, // membershipVersion — use 0 for now, dispatcher validates
		local.NodeID,
		0, // sourceSeq
		PeerCategoryQuery,
		"task.scatter",
		payload,
	)
	if err != nil {
		return
	}
	msg.Route = RoutePlan{Mode: RouteModeBroadcast}
	msg.CorrelationID = task.CorrelationID

	if err := SignClusterPeerMessage(local, msg); err != nil {
		return
	}

	secret, err := m.secretProvider.GetSecret()
	if err != nil {
		return
	}
	token, err := DecryptClusterDomainToken(secret, member.PeerTokenEncrypted)
	if err != nil {
		return
	}

	_ = m.delivery.Send(context.Background(), msg, *member, token)
}

func (m *DomainTaskManager) retryUnresponsive(task *ScatterGatherTask) {
	// Find nodes that haven't responded
	members, err := m.resolveDomainMembers()
	if err != nil {
		return
	}
	for _, member := range members {
		if _, ok := task.Results[member.NodeID]; !ok {
			m.sendScatterBroadcast(task, &member)
		}
	}
}

func (m *DomainTaskManager) markUnresponsiveTimeout(task *ScatterGatherTask) {
	members, err := m.resolveDomainMembers()
	if err != nil {
		return
	}
	for _, member := range members {
		if _, ok := task.Results[member.NodeID]; !ok {
			task.Results[member.NodeID] = NodeResult{
				NodeID: member.NodeID,
				Status: "timeout",
			}
		}
	}
}

// DeliverResult routes an incoming result from a remote node to the matching active task.
func (m *DomainTaskManager) DeliverResult(taskID string, correlationID string, result NodeResult) {
	m.mu.RLock()
	task, ok := m.active[taskID]
	m.mu.RUnlock()
	if !ok {
		return
	}
	select {
	case task.ResultChan <- result:
	default:
	}
}

// GetActiveTasks returns all currently executing tasks for this domain.
func (m *DomainTaskManager) GetActiveTasks() []*ScatterGatherTask {
	m.mu.RLock()
	defer m.mu.RUnlock()
	tasks := make([]*ScatterGatherTask, 0, len(m.active))
	for _, t := range m.active {
		tasks = append(tasks, t)
	}
	return tasks
}

// WorkerUtilization reports how many workers are busy vs total.
func (m *DomainTaskManager) WorkerUtilization() (active int, total int) {
	m.mu.RLock()
	defer m.mu.RUnlock()
	return len(m.active), m.workers
}

// ScatterGatherManager is the top-level orchestrator that owns per-domain task managers.
type ScatterGatherManager struct {
	managers       map[string]*DomainTaskManager
	mu             sync.RWMutex
	db             *gorm.DB
	delivery       *ClusterPeerDeliveryService
	identity       ClusterLocalIdentityService
	syncStore      clusterSyncStore
	secretProvider clusterSecretProvider
	hubClient      clusterHubClient
	hubURL         string
}

// NewScatterGatherManager creates a new ScatterGatherManager.
func NewScatterGatherManager(
	db *gorm.DB,
	delivery *ClusterPeerDeliveryService,
	identity ClusterLocalIdentityService,
	syncStore clusterSyncStore,
	secretProvider clusterSecretProvider,
) *ScatterGatherManager {
	return &ScatterGatherManager{
		managers:       make(map[string]*DomainTaskManager),
		db:             db,
		delivery:       delivery,
		identity:       identity,
		syncStore:      syncStore,
		secretProvider: secretProvider,
	}
}

// SetHubClient configures the Hub client for domain-scoped reporting.
func (m *ScatterGatherManager) SetHubClient(hubClient clusterHubClient, hubURL string) {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.hubClient = hubClient
	m.hubURL = hubURL
}

// GetOrCreateDomainManager returns the existing DomainTaskManager for a domain,
// or creates one with 2 workers and a buffered queue.
func (m *ScatterGatherManager) GetOrCreateDomainManager(domainID string) *DomainTaskManager {
	m.mu.Lock()
	defer m.mu.Unlock()
	if mgr, ok := m.managers[domainID]; ok {
		return mgr
	}
	mgr := &DomainTaskManager{
		domainID:       domainID,
		queue:          make(chan *ScatterGatherTask, 100),
		workers:        2,
		active:         make(map[string]*ScatterGatherTask),
		db:             m.db,
		delivery:       m.delivery,
		identity:       m.identity,
		syncStore:      m.syncStore,
		secretProvider: m.secretProvider,
		hubClient:      m.hubClient,
		hubURL:         m.hubURL,
	}
	m.managers[domainID] = mgr
	for i := 0; i < mgr.workers; i++ {
		go mgr.worker()
	}
	return mgr
}

// TaskSummary is the API response type for a scatter-gather task.
type TaskSummary struct {
	TaskID      string `json:"taskId"`
	TaskType    string `json:"taskType"`
	Status      string `json:"status"`
	Scope       string `json:"scope"`
	Progress    string `json:"progress"`
	CreatedAt   string `json:"createdAt"`
	CompletedAt string `json:"completedAt,omitempty"`
}

// TaskResultDetail is the API response type for a completed task result.
type TaskResultDetail struct {
	TaskID      string `json:"taskId"`
	TaskType    string `json:"taskType"`
	Status      string `json:"status"`
	Result      any    `json:"result"`
	GeneratedAt string `json:"generatedAt,omitempty"`
}

func mustMarshalJSON(v any) string {
	data, _ := json.Marshal(v)
	return string(data)
}
