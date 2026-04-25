package service

import (
	"fmt"
	"testing"

	"github.com/alireza0/s-ui/src/backend/internal/infra/db/model"
)

type stubClusterReachabilityStore struct {
	entries map[string]*model.ClusterPeerReachability
}

func newStubClusterReachabilityStore() *stubClusterReachabilityStore {
	return &stubClusterReachabilityStore{entries: map[string]*model.ClusterPeerReachability{}}
}

func (s *stubClusterReachabilityStore) GetReachability(domainID uint, targetNodeID string) (*model.ClusterPeerReachability, error) {
	entry := s.entries[s.key(domainID, targetNodeID)]
	if entry == nil {
		return nil, nil
	}
	copy := *entry
	return &copy, nil
}

func (s *stubClusterReachabilityStore) SaveReachability(entry *model.ClusterPeerReachability) error {
	copy := *entry
	s.entries[s.key(entry.DomainID, entry.TargetNodeID)] = &copy
	return nil
}

func (s *stubClusterReachabilityStore) DeleteReachabilityByDomain(domainID uint) error {
	for key, entry := range s.entries {
		if entry.DomainID == domainID {
			delete(s.entries, key)
		}
	}
	return nil
}

func (s *stubClusterReachabilityStore) DeleteReachabilityNotInTargets(domainID uint, targetNodeIDs []string) error {
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
