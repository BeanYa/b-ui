package service

import (
	"fmt"
	"sync"
	"testing"
	"time"

	"github.com/alireza0/s-ui/src/backend/internal/infra/db/model"
)

func TestClusterReachabilityTransitionsAcrossFailuresAndRecovery(t *testing.T) {
	current := time.Unix(1_700_000_000, 0)
	service := &ClusterReachabilityService{
		store:  newStubClusterReachabilityStore(),
		policy: DefaultClusterReachabilityPolicy(),
		now: func() int64 {
			return current.Unix()
		},
	}

	success, err := service.RecordTransportSuccess(7, "node-b", "runtime")
	if err != nil {
		t.Fatalf("record transport success: %v", err)
	}
	if success.State != ClusterReachabilityReachable {
		t.Fatalf("expected reachable after success, got %q", success.State)
	}
	if success.ConsecutiveFailures != 0 {
		t.Fatalf("expected failures reset after success, got %d", success.ConsecutiveFailures)
	}

	current = current.Add(5 * time.Second)
	suspect, err := service.RecordTransportFailure(7, "node-b", "runtime")
	if err != nil {
		t.Fatalf("record first transport failure: %v", err)
	}
	if suspect.State != ClusterReachabilitySuspect {
		t.Fatalf("expected suspect after first failure, got %q", suspect.State)
	}
	if suspect.ConsecutiveFailures != 1 {
		t.Fatalf("expected one consecutive failure, got %d", suspect.ConsecutiveFailures)
	}

	current = current.Add(10 * time.Second)
	_, err = service.RecordTransportFailure(7, "node-b", "runtime")
	if err != nil {
		t.Fatalf("record second transport failure: %v", err)
	}

	current = current.Add(30 * time.Second)
	unreachable, err := service.RecordTransportFailure(7, "node-b", "runtime")
	if err != nil {
		t.Fatalf("record third transport failure: %v", err)
	}
	if unreachable.State != ClusterReachabilityUnreachable {
		t.Fatalf("expected unreachable after threshold, got %q", unreachable.State)
	}
	if unreachable.ConsecutiveFailures != 3 {
		t.Fatalf("expected three consecutive failures, got %d", unreachable.ConsecutiveFailures)
	}
	expectedProbe := current.Add(60 * time.Second).Unix()
	if unreachable.NextProbeAt != expectedProbe {
		t.Fatalf("expected next probe at %d, got %d", expectedProbe, unreachable.NextProbeAt)
	}

	current = current.Add(61 * time.Second)
	recovered, err := service.RecordTransportSuccess(7, "node-b", "runtime")
	if err != nil {
		t.Fatalf("record recovery transport success: %v", err)
	}
	if recovered.State != ClusterReachabilityReachable {
		t.Fatalf("expected reachable after recovery, got %q", recovered.State)
	}
	if recovered.ConsecutiveFailures != 0 {
		t.Fatalf("expected failures reset after recovery, got %d", recovered.ConsecutiveFailures)
	}
	if recovered.NextProbeAt != 0 {
		t.Fatalf("expected next probe to clear after recovery, got %d", recovered.NextProbeAt)
	}
}

func TestClusterReachabilitySchedulesIdleProbeOnlyAfterSilenceWindow(t *testing.T) {
	current := time.Unix(1_700_000_000, 0)
	store := newStubClusterReachabilityStore()
	service := &ClusterReachabilityService{
		store:  store,
		policy: DefaultClusterReachabilityPolicy(),
		now: func() int64 {
			return current.Unix()
		},
	}

	entry, err := service.RecordTransportSuccess(9, "node-c", "runtime")
	if err != nil {
		t.Fatalf("seed reachability success: %v", err)
	}
	loaded, err := store.GetReachability(9, "node-c")
	if err != nil {
		t.Fatalf("load stored reachability: %v", err)
	}
	if loaded.State != ClusterReachabilityReachable {
		t.Fatalf("expected persisted reachable state, got %q", loaded.State)
	}

	shouldProbe, err := service.shouldProbeWithError(entry)
	if err != nil {
		t.Fatalf("should probe for recent observation: %v", err)
	}
	if shouldProbe {
		t.Fatal("expected recent observation not to be probe eligible")
	}

	current = current.Add(service.policy.IdleProbeAfter - time.Second)
	shouldProbe, err = service.shouldProbeWithError(entry)
	if err != nil {
		t.Fatalf("should probe before silence window: %v", err)
	}
	if shouldProbe {
		t.Fatal("expected peer to remain ineligible before silence window expires")
	}

	current = current.Add(2 * time.Second)
	shouldProbe, err = service.shouldProbeWithError(entry)
	if err != nil {
		t.Fatalf("should probe after silence window: %v", err)
	}
	if !shouldProbe {
		t.Fatal("expected peer to become probe eligible after silence window")
	}
	if entry.State != ClusterReachabilityReachable {
		t.Fatalf("expected state to remain reachable before unknown silence threshold, got %q", entry.State)
	}

	current = current.Add(service.policy.UnknownAfterSilence)
	shouldProbe, err = service.shouldProbeWithError(entry)
	if err != nil {
		t.Fatalf("should probe after stale silence: %v", err)
	}
	if !shouldProbe {
		t.Fatal("expected stale peer to stay probe eligible")
	}
	if entry.State != ClusterReachabilityUnknown {
		t.Fatalf("expected stale peer to downgrade to unknown, got %q", entry.State)
	}
	persisted, err := store.GetReachability(9, "node-c")
	if err != nil {
		t.Fatalf("load stale persisted reachability: %v", err)
	}
	if persisted.State != ClusterReachabilityUnknown {
		t.Fatalf("expected persisted state to downgrade to unknown, got %q", persisted.State)
	}
}

func TestClusterReachabilityClearsPeerRowsForSingleNodeDomains(t *testing.T) {
	store := newStubClusterReachabilityStore()
	service := &ClusterReachabilityService{
		store:  store,
		policy: DefaultClusterReachabilityPolicy(),
		now: func() int64 {
			return time.Unix(1_700_000_000, 0).Unix()
		},
	}

	if _, err := service.RecordTransportSuccess(11, "node-a", "runtime"); err != nil {
		t.Fatalf("seed node-a reachability: %v", err)
	}
	if _, err := service.RecordTransportFailure(11, "node-b", "runtime"); err != nil {
		t.Fatalf("seed node-b reachability: %v", err)
	}
	if len(store.entries) != 2 {
		t.Fatalf("expected two reachability rows before reconcile, got %d", len(store.entries))
	}

	if err := service.ReconcileMembers(11, []string{"node-self"}); err != nil {
		t.Fatalf("reconcile single-node domain: %v", err)
	}
	if len(store.entries) != 0 {
		t.Fatalf("expected peer rows to be cleared for single-node domain, got %d", len(store.entries))
	}
}

type stubClusterReachabilityStore struct {
	mu      sync.Mutex
	entries map[string]*model.ClusterPeerReachability
}

func newStubClusterReachabilityStore() *stubClusterReachabilityStore {
	return &stubClusterReachabilityStore{entries: map[string]*model.ClusterPeerReachability{}}
}

func (s *stubClusterReachabilityStore) GetReachability(domainID uint, targetNodeID string) (*model.ClusterPeerReachability, error) {
	s.mu.Lock()
	defer s.mu.Unlock()

	entry := s.entries[s.key(domainID, targetNodeID)]
	if entry == nil {
		return nil, nil
	}
	copy := *entry
	return &copy, nil
}

func (s *stubClusterReachabilityStore) SaveReachability(entry *model.ClusterPeerReachability) error {
	s.mu.Lock()
	defer s.mu.Unlock()

	copy := *entry
	s.entries[s.key(entry.DomainID, entry.TargetNodeID)] = &copy
	return nil
}

func (s *stubClusterReachabilityStore) DeleteReachabilityByDomain(domainID uint) error {
	s.mu.Lock()
	defer s.mu.Unlock()

	for key, entry := range s.entries {
		if entry.DomainID == domainID {
			delete(s.entries, key)
		}
	}
	return nil
}

func (s *stubClusterReachabilityStore) DeleteReachabilityNotInTargets(domainID uint, targetNodeIDs []string) error {
	s.mu.Lock()
	defer s.mu.Unlock()

	allowed := map[string]struct{}{}
	for _, targetNodeID := range targetNodeIDs {
		allowed[targetNodeID] = struct{}{}
	}
	for key, entry := range s.entries {
		if entry.DomainID != domainID {
			continue
		}
		if _, ok := allowed[entry.TargetNodeID]; !ok {
			delete(s.entries, key)
		}
	}
	return nil
}

func (s *stubClusterReachabilityStore) key(domainID uint, targetNodeID string) string {
	return fmt.Sprintf("%d/%s", domainID, targetNodeID)
}

type delayedClusterReachabilityStore struct {
	*stubClusterReachabilityStore
	saveDelay time.Duration
}

func (s *delayedClusterReachabilityStore) SaveReachability(entry *model.ClusterPeerReachability) error {
	time.Sleep(s.saveDelay)
	return s.stubClusterReachabilityStore.SaveReachability(entry)
}

func TestClusterReachabilityUpsertClauseTargetsDomainAndNode(t *testing.T) {
	upsert := clusterReachabilityUpsertClause()

	if len(upsert.Columns) != 2 {
		t.Fatalf("expected two conflict columns, got %d", len(upsert.Columns))
	}
	if upsert.Columns[0].Name != "domain_id" || upsert.Columns[1].Name != "target_node_id" {
		t.Fatalf("expected conflict key on domain_id/target_node_id, got %#v", upsert.Columns)
	}

	assignments := upsert.DoUpdates
	expected := []string{
		"state",
		"last_observed_at",
		"last_success_at",
		"last_failure_at",
		"consecutive_failures",
		"next_probe_at",
		"last_observation_source",
	}
	if len(assignments) != len(expected) {
		t.Fatalf("expected %d updated columns, got %d", len(expected), len(assignments))
	}
	for index, name := range expected {
		if assignments[index].Column.Name != name {
			t.Fatalf("expected assignment %d to target %q, got %q", index, name, assignments[index].Column.Name)
		}
	}
}

func TestClusterReachabilitySerializesConcurrentFailureMutations(t *testing.T) {
	store := &delayedClusterReachabilityStore{
		stubClusterReachabilityStore: newStubClusterReachabilityStore(),
		saveDelay:                    2 * time.Millisecond,
	}
	service := &ClusterReachabilityService{
		store:  store,
		policy: DefaultClusterReachabilityPolicy(),
		now: func() int64 {
			return time.Unix(1_700_000_000, 0).Unix()
		},
	}

	const failures = 16
	start := make(chan struct{})
	errs := make(chan error, failures)
	var wg sync.WaitGroup

	for index := 0; index < failures; index++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			<-start
			_, err := service.RecordTransportFailure(21, "node-b", "runtime")
			errs <- err
		}()
	}

	close(start)
	wg.Wait()
	close(errs)

	for err := range errs {
		if err != nil {
			t.Fatalf("record concurrent failure: %v", err)
		}
	}

	entry, err := store.GetReachability(21, "node-b")
	if err != nil {
		t.Fatalf("load concurrent reachability: %v", err)
	}
	if entry == nil {
		t.Fatal("expected persisted reachability after concurrent failures")
	}
	if entry.ConsecutiveFailures != failures {
		t.Fatalf("expected %d consecutive failures after concurrent updates, got %d", failures, entry.ConsecutiveFailures)
	}
	if entry.State != ClusterReachabilityUnreachable {
		t.Fatalf("expected unreachable state after concurrent failures, got %q", entry.State)
	}
}
