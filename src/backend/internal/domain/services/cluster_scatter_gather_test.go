package service

import (
	"context"
	"encoding/json"
	"testing"
	"time"

	"github.com/BeanYa/b-ui/src/backend/internal/infra/db/model"
)

// --- Handler registration ---

func TestRegisterAndGetScatterHandler(t *testing.T) {
	h := &stubScatterHandler{taskType: "test.echo"}
	RegisterScatterHandler(h)
	got, ok := GetScatterHandler("test.echo")
	if !ok {
		t.Fatal("expected handler to be registered")
	}
	if got.TaskType() != "test.echo" {
		t.Fatalf("expected taskType test.echo, got %q", got.TaskType())
	}
}

func TestGetScatterHandlerMissing(t *testing.T) {
	_, ok := GetScatterHandler("nonexistent.handler")
	if ok {
		t.Fatal("expected missing handler to return false")
	}
}

func TestMeshLatencyHandlerRegisteredByInit(t *testing.T) {
	h, ok := GetScatterHandler(TaskTypeMeshLatency)
	if !ok {
		t.Fatal("expected mesh.latency handler to be registered by init()")
	}
	if h.TaskType() != TaskTypeMeshLatency {
		t.Fatalf("expected taskType %q, got %q", TaskTypeMeshLatency, h.TaskType())
	}
}

// --- NodeResult ---

func TestNodeResultJSONRoundTrip(t *testing.T) {
	original := NodeResult{
		NodeID: "node-1",
		Status: "completed",
		Result: map[string]any{"rtt_ms": 42.5},
	}
	data, err := json.Marshal(original)
	if err != nil {
		t.Fatalf("marshal: %v", err)
	}
	var decoded NodeResult
	if err := json.Unmarshal(data, &decoded); err != nil {
		t.Fatalf("unmarshal: %v", err)
	}
	if decoded.NodeID != "node-1" || decoded.Status != "completed" {
		t.Fatalf("round trip mismatch: %+v", decoded)
	}
}

// --- TaskSummary / TaskResultDetail ---

func TestTaskSummaryJSON(t *testing.T) {
	ts := TaskSummary{
		TaskID:      "task-abc",
		TaskType:    "mesh.latency",
		Status:      "completed",
		Scope:       "domain",
		Progress:    "3/3",
		CreatedAt:   "2026-05-01T10:00:00Z",
		CompletedAt: "2026-05-01T10:00:05Z",
	}
	data, err := json.Marshal(ts)
	if err != nil {
		t.Fatalf("marshal: %v", err)
	}
	var decoded TaskSummary
	if err := json.Unmarshal(data, &decoded); err != nil {
		t.Fatalf("unmarshal: %v", err)
	}
	if decoded.TaskID != "task-abc" {
		t.Fatalf("expected taskId task-abc, got %q", decoded.TaskID)
	}
	if decoded.CompletedAt != "2026-05-01T10:00:05Z" {
		t.Fatalf("completedAt mismatch: %q", decoded.CompletedAt)
	}
}

func TestTaskResultDetailJSON(t *testing.T) {
	tr := TaskResultDetail{
		TaskID:      "task-xyz",
		TaskType:    "mesh.latency",
		Status:      "completed",
		Result:      map[string]any{"matrix": map[string]any{}},
		GeneratedAt: "2026-05-01T10:00:10Z",
	}
	data, err := json.Marshal(tr)
	if err != nil {
		t.Fatalf("marshal: %v", err)
	}
	var decoded TaskResultDetail
	if err := json.Unmarshal(data, &decoded); err != nil {
		t.Fatalf("unmarshal: %v", err)
	}
	if decoded.TaskID != "task-xyz" {
		t.Fatalf("expected taskId task-xyz, got %q", decoded.TaskID)
	}
}

// --- DomainTaskManager ---

func TestDomainTaskManagerDeliverResult(t *testing.T) {
	mgr := &DomainTaskManager{
		domainID: "test.example.com",
		active:   make(map[string]*ScatterGatherTask),
	}

	task := &ScatterGatherTask{
		TaskID:     "task-r1",
		DomainID:   "test.example.com",
		TaskType:   "mesh.latency",
		Scope:      "domain",
		Params:     map[string]any{},
		CreatedAt:  time.Now(),
		Results:    make(map[string]NodeResult),
		ResultChan: make(chan NodeResult, 10),
	}
	mgr.active["task-r1"] = task

	mgr.DeliverResult("task-r1", "corr-1", NodeResult{NodeID: "node-a", Status: "completed"})

	select {
	case result := <-task.ResultChan:
		if result.NodeID != "node-a" {
			t.Fatalf("expected node-a, got %q", result.NodeID)
		}
	case <-time.After(100 * time.Millisecond):
		t.Fatal("expected result to be delivered to task channel")
	}
}

func TestDomainTaskManagerDeliverResultMissingTask(t *testing.T) {
	mgr := &DomainTaskManager{
		domainID: "test.example.com",
		active:   make(map[string]*ScatterGatherTask),
	}
	mgr.DeliverResult("nonexistent", "corr-1", NodeResult{NodeID: "node-a", Status: "completed"})
}

func TestDomainTaskManagerGetActiveTasks(t *testing.T) {
	mgr := &DomainTaskManager{
		domainID: "test.example.com",
		active:   make(map[string]*ScatterGatherTask),
	}
	task1 := &ScatterGatherTask{TaskID: "t1", DomainID: "test.example.com"}
	task2 := &ScatterGatherTask{TaskID: "t2", DomainID: "test.example.com"}
	mgr.active["t1"] = task1
	mgr.active["t2"] = task2

	tasks := mgr.GetActiveTasks()
	if len(tasks) != 2 {
		t.Fatalf("expected 2 active tasks, got %d", len(tasks))
	}
}

func TestDomainTaskManagerWorkerUtilization(t *testing.T) {
	mgr := &DomainTaskManager{
		domainID: "test.example.com",
		workers:  4,
		active:   make(map[string]*ScatterGatherTask),
	}
	mgr.active["t1"] = &ScatterGatherTask{TaskID: "t1"}

	active, total := mgr.WorkerUtilization()
	if active != 1 || total != 4 {
		t.Fatalf("expected 1/4, got %d/%d", active, total)
	}
}

// --- ScatterGatherManager ---

func TestScatterGatherManagerGetOrCreateDomainManager(t *testing.T) {
	store := &stubClusterSyncStore{
		domains: map[uint]*model.ClusterDomain{},
		members: map[string]*model.ClusterMember{},
	}
	mgr := NewScatterGatherManager(nil, nil, ClusterLocalIdentityService{}, store, &stubSecretProvider{})

	dm := mgr.GetOrCreateDomainManager("domain-a")
	if dm == nil {
		t.Fatal("expected domain manager to be created")
	}
	if dm.domainID != "domain-a" {
		t.Fatalf("expected domain-a, got %q", dm.domainID)
	}

	dm2 := mgr.GetOrCreateDomainManager("domain-a")
	if dm != dm2 {
		t.Fatal("expected same DomainTaskManager instance for same domain")
	}

	dm3 := mgr.GetOrCreateDomainManager("domain-b")
	if dm == dm3 {
		t.Fatal("expected different DomainTaskManager for different domain")
	}
}

func TestScatterGatherManagerSetHubClient(t *testing.T) {
	store := &stubClusterSyncStore{
		domains: map[uint]*model.ClusterDomain{},
		members: map[string]*model.ClusterMember{},
	}
	mgr := NewScatterGatherManager(nil, nil, ClusterLocalIdentityService{}, store, &stubSecretProvider{})
	hubClient := &stubHubClient{}
	mgr.SetHubClient(hubClient, "https://hub.example.com")

	if mgr.hubClient != hubClient {
		t.Fatal("expected hub client to be set")
	}
	if mgr.hubURL != "https://hub.example.com" {
		t.Fatalf("expected hub URL, got %q", mgr.hubURL)
	}
}

// --- MeshLatencyHandler aggregate ---

func TestMeshLatencyHandlerAggregateEmpty(t *testing.T) {
	h := &MeshLatencyHandler{}
	result, err := h.Aggregate(context.Background(), nil)
	if err != nil {
		t.Fatalf("aggregate: %v", err)
	}
	report, ok := result.(*MeshLatencyReport)
	if !ok {
		t.Fatalf("expected *MeshLatencyReport, got %T", result)
	}
	if len(report.Matrix) != 0 {
		t.Fatalf("expected empty matrix, got %d entries", len(report.Matrix))
	}
}

func TestMeshLatencyHandlerAggregateWithResults(t *testing.T) {
	h := &MeshLatencyHandler{}

	latency1 := 12.5
	latency2 := 8.3
	latency3 := 15.0
	latency4 := 9.7

	results := []NodeResult{
		{
			NodeID: "node-a",
			Status: "completed",
			Result: map[string]any{
				"latencies": map[string]any{
					"node-b": map[string]any{"rtt_ms": latency1},
					"node-c": map[string]any{"rtt_ms": latency2},
				},
			},
		},
		{
			NodeID: "node-b",
			Status: "completed",
			Result: map[string]any{
				"latencies": map[string]any{
					"node-a": map[string]any{"rtt_ms": latency3},
					"node-c": map[string]any{"rtt_ms": latency4},
				},
			},
		},
	}

	aggregated, err := h.Aggregate(context.Background(), results)
	if err != nil {
		t.Fatalf("aggregate: %v", err)
	}

	report := aggregated.(*MeshLatencyReport)
	if len(report.Matrix) != 2 {
		t.Fatalf("expected 2 nodes in matrix, got %d", len(report.Matrix))
	}
	if len(report.NodeSummary) != 2 {
		t.Fatalf("expected 2 node summaries, got %d", len(report.NodeSummary))
	}

	summaryA := report.NodeSummary["node-a"]
	if summaryA.Reachable != 2 || summaryA.Total != 2 {
		t.Fatalf("node-a summary: reachable=%d total=%d", summaryA.Reachable, summaryA.Total)
	}

	val := report.Matrix["node-a"]["node-b"]
	if val == nil || *val != 12.5 {
		t.Fatalf("expected 12.5 for node-a->node-b, got %v", val)
	}
}

func TestMeshLatencyHandlerAggregateWithFailedNode(t *testing.T) {
	h := &MeshLatencyHandler{}

	latency := 20.0
	results := []NodeResult{
		{
			NodeID: "node-a",
			Status: "completed",
			Result: map[string]any{
				"latencies": map[string]any{
					"node-b": map[string]any{"rtt_ms": latency},
				},
			},
		},
		{
			NodeID: "node-b",
			Status: "failed",
			Error:  "connection refused",
		},
	}

	aggregated, err := h.Aggregate(context.Background(), results)
	if err != nil {
		t.Fatalf("aggregate: %v", err)
	}

	report := aggregated.(*MeshLatencyReport)
	summaryB := report.NodeSummary["node-b"]
	if summaryB.Reachable != 0 || summaryB.Total != 0 {
		t.Fatalf("node-b summary: reachable=%d total=%d", summaryB.Reachable, summaryB.Total)
	}
}

// --- MeshLatencyReport JSON round-trip ---

func TestMeshLatencyReportJSONRoundTrip(t *testing.T) {
	latency := 42.0
	report := &MeshLatencyReport{
		Matrix: map[string]map[string]*float64{
			"node-a": {"node-b": &latency},
		},
		NodeSummary: map[string]NodeLatencySummary{
			"node-a": {AvgMs: 42.0, Reachable: 1, Total: 1},
		},
		GeneratedAt: "2026-05-01T10:00:00Z",
	}
	data, err := json.Marshal(report)
	if err != nil {
		t.Fatalf("marshal: %v", err)
	}

	var decoded map[string]any
	if err := json.Unmarshal(data, &decoded); err != nil {
		t.Fatalf("unmarshal: %v", err)
	}

	matrix := decoded["matrix"].(map[string]any)
	if matrix["node-a"] == nil {
		t.Fatal("expected matrix to contain node-a")
	}

	nodeSummary := decoded["node_summary"].(map[string]any)
	if nodeSummary["node-a"] == nil {
		t.Fatal("expected node_summary to contain node-a")
	}
}

// --- TaskStatus constants ---

func TestTaskStatusValues(t *testing.T) {
	if TaskStatusCreated != "created" {
		t.Fatalf("expected created, got %q", TaskStatusCreated)
	}
	if TaskStatusCompleted != "completed" {
		t.Fatalf("expected completed, got %q", TaskStatusCompleted)
	}
	if TaskStatusFailed != "failed" {
		t.Fatalf("expected failed, got %q", TaskStatusFailed)
	}
	if TaskStatusTimeout != "timeout" {
		t.Fatalf("expected timeout, got %q", TaskStatusTimeout)
	}
}

// --- mustMarshalJSON ---

func TestMustMarshalJSON(t *testing.T) {
	result := mustMarshalJSON(map[string]any{"key": "value"})
	if result != `{"key":"value"}` {
		t.Fatalf("expected {\"key\":\"value\"}, got %q", result)
	}
}

func TestMustMarshalJSONNil(t *testing.T) {
	result := mustMarshalJSON(nil)
	if result != "null" {
		t.Fatalf("expected null, got %q", result)
	}
}

// --- stubs ---

type stubScatterHandler struct {
	taskType string
}

func (s *stubScatterHandler) TaskType() string { return s.taskType }
func (s *stubScatterHandler) ExecuteLocal(_ context.Context, _ string, params map[string]any) (any, error) {
	return map[string]any{"echo": params}, nil
}
func (s *stubScatterHandler) Aggregate(_ context.Context, results []NodeResult) (any, error) {
	return map[string]any{"count": len(results)}, nil
}

type stubSecretProvider struct{}

func (s *stubSecretProvider) GetSecret() ([]byte, error) {
	return []byte("test-secret"), nil
}

type stubHubClient struct{}

func (s *stubHubClient) RegisterNode(_ context.Context, _ string, _ ClusterHubRegisterNodeRequest) (*ClusterHubOperationResponse, error) {
	return nil, nil
}
func (s *stubHubClient) GetLatestVersion(_ context.Context, _ string, _ string, _ string) (*ClusterHubVersionResponse, error) {
	return nil, nil
}
func (s *stubHubClient) GetSnapshot(_ context.Context, _ string, _ string, _ string) (*ClusterHubSnapshotResponse, error) {
	return nil, nil
}
func (s *stubHubClient) DeleteMember(_ context.Context, _ string, _ string, _ string, _ string, _ bool) (*ClusterHubOperationResponse, error) {
	return nil, nil
}
func (s *stubHubClient) ClaimUpdate(_ context.Context, _ string, _ string, _ string, _ string, _ string) (*ClusterHubClaimUpdateResponse, error) {
	return nil, nil
}
func (s *stubHubClient) SetMemberStatus(_ context.Context, _ string, _ string, _ string, _ string, _ string, _ string, _ string) (*ClusterHubMemberStatusResponse, error) {
	return nil, nil
}
func (s *stubHubClient) ReportProxyConfigs(_ context.Context, _ string, _ string, _ ClusterHubReportProxyConfigsRequest) error {
	return nil
}
func (s *stubHubClient) ReportDomainReport(_ context.Context, _ string, _ string, _ ClusterHubReportRequest) error {
	return nil
}
func (s *stubHubClient) ReportDomainResourceState(_ context.Context, _ string, _ string, _ string, _ ClusterHubResourceStateReportRequest) error {
	return nil
}

var _ clusterSecretProvider = (*stubSecretProvider)(nil)
var _ clusterHubClient = (*stubHubClient)(nil)
