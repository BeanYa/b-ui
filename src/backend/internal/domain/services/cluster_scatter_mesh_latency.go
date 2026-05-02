package service

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"sort"
	"sync"
	"time"

	"github.com/BeanYa/b-ui/src/backend/internal/infra/db/model"
)

const TaskTypeMeshLatency = "mesh.latency"

func init() {
	RegisterScatterHandler(&MeshLatencyHandler{})
}

// MeshLatencyHandler implements ScatterGatherHandler for P2P mesh latency testing.
// Each node pings all other specified target nodes (or all domain members if no
// targets specified), measuring RTT with 3 HTTP GET probes and taking the median.
// The initiator then aggregates results into a full latency matrix.
type MeshLatencyHandler struct {
	identity       ClusterLocalIdentityService
	syncStore      clusterSyncStore
	secretProvider clusterSecretProvider
}

// NewMeshLatencyHandler creates a handler with its runtime dependencies and
// registers it globally, replacing any placeholder registered by init().
func NewMeshLatencyHandler(
	identity ClusterLocalIdentityService,
	syncStore clusterSyncStore,
	secretProvider clusterSecretProvider,
) *MeshLatencyHandler {
	h := &MeshLatencyHandler{
		identity:       identity,
		syncStore:      syncStore,
		secretProvider: secretProvider,
	}
	RegisterScatterHandler(h)
	return h
}

func (h *MeshLatencyHandler) TaskType() string {
	return TaskTypeMeshLatency
}

// TargetLatency holds the RTT result for a single target.
type TargetLatency struct {
	RttMs  *float64 `json:"rtt_ms"`
	Status string   `json:"status"` // "ok" or "timeout"
}

// MeshLatencyLocalResult is what each node returns from ExecuteLocal.
type MeshLatencyLocalResult struct {
	Latencies map[string]TargetLatency `json:"latencies"`
}

func (h *MeshLatencyHandler) ExecuteLocal(ctx context.Context, domainID string, params map[string]any) (any, error) {
	var targets []string
	if targetsRaw, ok := params["targets"].([]any); ok {
		for _, t := range targetsRaw {
			if s, ok := t.(string); ok {
				targets = append(targets, s)
			}
		}
	}

	local, err := h.identity.GetOrCreate()
	if err != nil {
		return nil, err
	}

	// If no targets specified, get all domain members except self.
	if len(targets) == 0 {
		members, err := h.resolveDomainMembers(domainID)
		if err != nil {
			return nil, err
		}
		for _, m := range members {
			if m.NodeID != local.NodeID {
				targets = append(targets, m.NodeID)
			}
		}
	}

	latencies := make(map[string]TargetLatency)
	var mu sync.Mutex
	var wg sync.WaitGroup

	for _, targetID := range targets {
		wg.Add(1)
		go func(tid string) {
			defer wg.Done()
			tl := h.probeTarget(ctx, domainID, tid)
			mu.Lock()
			latencies[tid] = tl
			mu.Unlock()
		}(targetID)
	}
	wg.Wait()

	return &MeshLatencyLocalResult{Latencies: latencies}, nil
}

// probeTarget sends 3 HTTP GET probes to a target node's ping endpoint and
// returns the median RTT. Individual probe timeout is 5 seconds.
func (h *MeshLatencyHandler) probeTarget(ctx context.Context, domainID string, targetID string) TargetLatency {
	member, err := h.lookupMember(domainID, targetID)
	if err != nil {
		return TargetLatency{Status: "timeout"}
	}
	secret, err := h.secretProvider.GetSecret()
	if err != nil {
		return TargetLatency{Status: "timeout"}
	}
	token, err := DecryptClusterDomainToken(secret, member.PeerTokenEncrypted)
	if err != nil {
		return TargetLatency{Status: "timeout"}
	}

	// 3 probes, take median.
	var rtts []float64
	for i := 0; i < 3; i++ {
		rtt, ok := h.singlePing(member.BaseURL, token, targetID)
		if ok {
			rtts = append(rtts, rtt)
		}
	}

	if len(rtts) == 0 {
		return TargetLatency{Status: "timeout"}
	}

	sort.Float64s(rtts)
	median := rtts[len(rtts)/2]
	return TargetLatency{RttMs: &median, Status: "ok"}
}

// singlePing performs one HTTP GET to the target's ping endpoint and returns
// the round-trip time in milliseconds.
func (h *MeshLatencyHandler) singlePing(baseURL string, token string, targetID string) (float64, bool) {
	client := &http.Client{Timeout: 5 * time.Second}
	url := fmt.Sprintf("%s/_cluster/v1/ping?node_id=%s", baseURL, targetID)
	req, err := http.NewRequest("GET", url, nil)
	if err != nil {
		return 0, false
	}
	req.Header.Set("X-Cluster-Token", token)
	start := time.Now()
	resp, err := client.Do(req)
	if err != nil {
		return 0, false
	}
	defer resp.Body.Close()
	elapsed := time.Since(start).Seconds() * 1000 // ms
	if resp.StatusCode == http.StatusOK {
		return elapsed, true
	}
	return 0, false
}

// lookupMember resolves a domain name to its DB ID, then fetches the member.
func (h *MeshLatencyHandler) lookupMember(domainID string, nodeID string) (*model.ClusterMember, error) {
	domains, err := h.syncStore.ListDomains()
	if err != nil {
		return nil, err
	}
	for _, d := range domains {
		if d.Domain == domainID {
			return h.syncStore.GetMember(d.Id, nodeID)
		}
	}
	return nil, fmt.Errorf("domain not found: %s", domainID)
}

// resolveDomainMembers resolves a domain name to its DB ID, then fetches all members.
func (h *MeshLatencyHandler) resolveDomainMembers(domainID string) ([]model.ClusterMember, error) {
	domains, err := h.syncStore.ListDomains()
	if err != nil {
		return nil, err
	}
	for _, d := range domains {
		if d.Domain == domainID {
			return h.syncStore.GetMembers(d.Id)
		}
	}
	return nil, fmt.Errorf("domain not found: %s", domainID)
}

// MeshLatencyReport is the final aggregated report produced by Aggregate().
type MeshLatencyReport struct {
	Matrix      map[string]map[string]*float64 `json:"matrix"`
	NodeSummary map[string]NodeLatencySummary  `json:"node_summary"`
	GeneratedAt string                         `json:"generated_at"`
}

// NodeLatencySummary holds aggregate stats for one node.
type NodeLatencySummary struct {
	AvgMs     float64 `json:"avg_ms"`
	Reachable int     `json:"reachable"`
	Total     int     `json:"total"`
}

func (h *MeshLatencyHandler) Aggregate(ctx context.Context, results []NodeResult) (any, error) {
	matrix := make(map[string]map[string]*float64)
	summaries := make(map[string]NodeLatencySummary)

	for _, nr := range results {
		matrix[nr.NodeID] = make(map[string]*float64)

		if nr.Status != "completed" || nr.Result == nil {
			summaries[nr.NodeID] = NodeLatencySummary{Reachable: 0, Total: 0}
			continue
		}

		localResult := &MeshLatencyLocalResult{}
		data, err := json.Marshal(nr.Result)
		if err != nil {
			summaries[nr.NodeID] = NodeLatencySummary{Reachable: 0, Total: 0}
			continue
		}
		if err := json.Unmarshal(data, localResult); err != nil {
			summaries[nr.NodeID] = NodeLatencySummary{Reachable: 0, Total: 0}
			continue
		}

		var totalRtt float64
		reachable := 0
		totalTargets := len(localResult.Latencies)

		for targetID, tl := range localResult.Latencies {
			matrix[nr.NodeID][targetID] = tl.RttMs
			if tl.RttMs != nil {
				totalRtt += *tl.RttMs
				reachable++
			}
		}

		avg := 0.0
		if reachable > 0 {
			avg = totalRtt / float64(reachable)
		}
		summaries[nr.NodeID] = NodeLatencySummary{
			AvgMs:     avg,
			Reachable: reachable,
			Total:     totalTargets,
		}
	}

	return &MeshLatencyReport{
		Matrix:      matrix,
		NodeSummary: summaries,
		GeneratedAt: time.Now().UTC().Format(time.RFC3339),
	}, nil
}
