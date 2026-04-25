package service

import (
	"testing"
	"time"
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

	shouldProbe, err := service.ShouldProbeWithError(entry)
	if err != nil {
		t.Fatalf("should probe for recent observation: %v", err)
	}
	if shouldProbe {
		t.Fatal("expected recent observation not to be probe eligible")
	}

	current = current.Add(service.policy.IdleProbeAfter - time.Second)
	shouldProbe, err = service.ShouldProbeWithError(entry)
	if err != nil {
		t.Fatalf("should probe before silence window: %v", err)
	}
	if shouldProbe {
		t.Fatal("expected peer to remain ineligible before silence window expires")
	}

	current = current.Add(2 * time.Second)
	shouldProbe, err = service.ShouldProbeWithError(entry)
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
	shouldProbe, err = service.ShouldProbeWithError(entry)
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
