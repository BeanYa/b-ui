package service

import (
	"errors"
	"fmt"
	"time"

	database "github.com/alireza0/s-ui/src/backend/internal/infra/db"
	"github.com/alireza0/s-ui/src/backend/internal/infra/db/model"
)

const ClusterReachabilityUnknown = "unknown"
const ClusterReachabilityReachable = "reachable"
const ClusterReachabilitySuspect = "suspect"
const ClusterReachabilityUnreachable = "unreachable"

type ClusterReachabilityPolicy struct {
	IdleProbeAfter           time.Duration
	ProbeInterval            time.Duration
	ProbeTimeout             time.Duration
	SuspectAfterFailures     int64
	UnreachableAfterFailures int64
	UnknownAfterSilence      time.Duration
	Backoff                  []time.Duration
}

func DefaultClusterReachabilityPolicy() ClusterReachabilityPolicy {
	return ClusterReachabilityPolicy{
		IdleProbeAfter:           30 * time.Second,
		ProbeInterval:            30 * time.Second,
		ProbeTimeout:             3 * time.Second,
		SuspectAfterFailures:     1,
		UnreachableAfterFailures: 3,
		UnknownAfterSilence:      10 * time.Minute,
		Backoff: []time.Duration{
			10 * time.Second,
			30 * time.Second,
			60 * time.Second,
			120 * time.Second,
		},
	}
}

type clusterReachabilityStore interface {
	GetReachability(domainID uint, targetNodeID string) (*model.ClusterPeerReachability, error)
	SaveReachability(*model.ClusterPeerReachability) error
	DeleteReachabilityByDomain(domainID uint) error
	DeleteReachabilityNotInTargets(domainID uint, targetNodeIDs []string) error
}

type ClusterReachabilityService struct {
	store  clusterReachabilityStore
	policy ClusterReachabilityPolicy
	now    func() int64
}

func (s *ClusterReachabilityService) RecordTransportSuccess(domainID uint, targetNodeID string, source string) (*model.ClusterPeerReachability, error) {
	entry, err := s.load(domainID, targetNodeID)
	if err != nil {
		return nil, err
	}
	now := s.currentUnix()
	entry.State = ClusterReachabilityReachable
	entry.LastObservedAt = now
	entry.LastSuccessAt = now
	entry.ConsecutiveFailures = 0
	entry.NextProbeAt = 0
	entry.LastObservationSource = source
	if err := s.getStore().SaveReachability(entry); err != nil {
		return nil, err
	}
	return entry, nil
}

func (s *ClusterReachabilityService) RecordTransportFailure(domainID uint, targetNodeID string, source string) (*model.ClusterPeerReachability, error) {
	entry, err := s.load(domainID, targetNodeID)
	if err != nil {
		return nil, err
	}
	now := s.currentUnix()
	entry.ConsecutiveFailures++
	entry.LastObservedAt = now
	entry.LastFailureAt = now
	entry.LastObservationSource = source
	if entry.ConsecutiveFailures >= s.policy.UnreachableAfterFailures {
		entry.State = ClusterReachabilityUnreachable
	} else if entry.ConsecutiveFailures >= s.policy.SuspectAfterFailures {
		entry.State = ClusterReachabilitySuspect
	}
	entry.NextProbeAt = now + int64(s.backoffForFailures(entry.ConsecutiveFailures)/time.Second)
	if err := s.getStore().SaveReachability(entry); err != nil {
		return nil, err
	}
	return entry, nil
}

func (s *ClusterReachabilityService) ShouldProbe(entry *model.ClusterPeerReachability) bool {
	if entry == nil {
		return false
	}
	now := s.currentUnix()
	if s.policy.UnknownAfterSilence > 0 && entry.LastObservedAt > 0 && now-entry.LastObservedAt >= int64(s.policy.UnknownAfterSilence/time.Second) {
		entry.State = ClusterReachabilityUnknown
	}
	if s.policy.IdleProbeAfter > 0 && entry.LastObservedAt > 0 && now-entry.LastObservedAt < int64(s.policy.IdleProbeAfter/time.Second) {
		return false
	}
	if entry.NextProbeAt > now {
		return false
	}
	return true
}

func (s *ClusterReachabilityService) ReconcileMembers(domainID uint, targetNodeIDs []string) error {
	if len(targetNodeIDs) <= 1 {
		return s.getStore().DeleteReachabilityByDomain(domainID)
	}
	return s.getStore().DeleteReachabilityNotInTargets(domainID, targetNodeIDs)
}

func (s *ClusterReachabilityService) load(domainID uint, targetNodeID string) (*model.ClusterPeerReachability, error) {
	entry, err := s.getStore().GetReachability(domainID, targetNodeID)
	if err != nil {
		return nil, err
	}
	if entry != nil {
		return entry, nil
	}
	return &model.ClusterPeerReachability{
		DomainID:     domainID,
		TargetNodeID: targetNodeID,
		State:        ClusterReachabilityUnknown,
	}, nil
}

func (s *ClusterReachabilityService) currentUnix() int64 {
	if s.now != nil {
		return s.now()
	}
	return time.Now().Unix()
}

func (s *ClusterReachabilityService) getStore() clusterReachabilityStore {
	if s.store != nil {
		return s.store
	}
	return &dbClusterReachabilityStore{}
}

func (s *ClusterReachabilityService) backoffForFailures(failures int64) time.Duration {
	if len(s.policy.Backoff) == 0 {
		return s.policy.ProbeInterval
	}
	index := int(failures - 1)
	if index < 0 {
		index = 0
	}
	if index >= len(s.policy.Backoff) {
		index = len(s.policy.Backoff) - 1
	}
	return s.policy.Backoff[index]
}

type dbClusterReachabilityStore struct{}

func (s *dbClusterReachabilityStore) GetReachability(domainID uint, targetNodeID string) (*model.ClusterPeerReachability, error) {
	entry := &model.ClusterPeerReachability{}
	err := database.GetDB().Where("domain_id = ? AND target_node_id = ?", domainID, targetNodeID).First(entry).Error
	if database.IsNotFound(err) {
		return nil, nil
	}
	if err != nil {
		return nil, err
	}
	return entry, nil
}

func (s *dbClusterReachabilityStore) SaveReachability(entry *model.ClusterPeerReachability) error {
	return database.GetDB().Save(entry).Error
}

func (s *dbClusterReachabilityStore) DeleteReachabilityByDomain(domainID uint) error {
	return database.GetDB().Where("domain_id = ?", domainID).Delete(&model.ClusterPeerReachability{}).Error
}

func (s *dbClusterReachabilityStore) DeleteReachabilityNotInTargets(domainID uint, targetNodeIDs []string) error {
	if len(targetNodeIDs) == 0 {
		return s.DeleteReachabilityByDomain(domainID)
	}
	return database.GetDB().Where("domain_id = ? AND target_node_id NOT IN ?", domainID, targetNodeIDs).Delete(&model.ClusterPeerReachability{}).Error
}

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
	if entry == nil {
		return errors.New("cluster reachability entry is nil")
	}
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
