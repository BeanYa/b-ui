package service

import (
	"sync"
	"time"

	database "github.com/alireza0/s-ui/src/backend/internal/infra/db"
	"github.com/alireza0/s-ui/src/backend/internal/infra/db/model"
	"gorm.io/gorm/clause"
)

const ClusterReachabilityUnknown = "unknown"
const ClusterReachabilityReachable = "reachable"
const ClusterReachabilitySuspect = "suspect"
const ClusterReachabilityUnreachable = "unreachable"

// Reachability state is persisted by a single local runtime process.
// Serializing mutations here avoids lost updates across concurrent goroutines.
var clusterReachabilityMutationMu sync.Mutex

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
	clusterReachabilityMutationMu.Lock()
	defer clusterReachabilityMutationMu.Unlock()

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
	clusterReachabilityMutationMu.Lock()
	defer clusterReachabilityMutationMu.Unlock()

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

// ShouldProbe preserves the original Task 1 API.
func (s *ClusterReachabilityService) ShouldProbe(entry *model.ClusterPeerReachability) bool {
	shouldProbe, err := s.shouldProbeWithError(entry)
	return err == nil && shouldProbe
}

func (s *ClusterReachabilityService) shouldProbeWithError(entry *model.ClusterPeerReachability) (bool, error) {
	clusterReachabilityMutationMu.Lock()
	defer clusterReachabilityMutationMu.Unlock()

	if entry == nil {
		return false, nil
	}
	now := s.currentUnix()
	if s.policy.UnknownAfterSilence > 0 && entry.LastObservedAt > 0 && now-entry.LastObservedAt >= int64(s.policy.UnknownAfterSilence/time.Second) {
		if entry.State != ClusterReachabilityUnknown {
			entry.State = ClusterReachabilityUnknown
			if err := s.getStore().SaveReachability(entry); err != nil {
				return false, err
			}
		}
	}
	if s.policy.IdleProbeAfter > 0 && entry.LastObservedAt > 0 && now-entry.LastObservedAt < int64(s.policy.IdleProbeAfter/time.Second) {
		return false, nil
	}
	if entry.NextProbeAt > now {
		return false, nil
	}
	return true, nil
}

// ReconcileMembers preserves the original Task 1 contract where targetNodeIDs
// includes the local node alongside any remote peers.
func (s *ClusterReachabilityService) ReconcileMembers(domainID uint, targetNodeIDs []string) error {
	clusterReachabilityMutationMu.Lock()
	defer clusterReachabilityMutationMu.Unlock()

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
	if err := database.GetDB().Clauses(clusterReachabilityUpsertClause()).Create(entry).Error; err != nil {
		return err
	}
	loaded, err := s.GetReachability(entry.DomainID, entry.TargetNodeID)
	if err != nil {
		return err
	}
	if loaded != nil {
		*entry = *loaded
	}
	return nil
}

func clusterReachabilityUpsertClause() clause.OnConflict {
	return clause.OnConflict{
		Columns: []clause.Column{
			{Name: "domain_id"},
			{Name: "target_node_id"},
		},
		DoUpdates: clause.AssignmentColumns([]string{
			"state",
			"last_observed_at",
			"last_success_at",
			"last_failure_at",
			"consecutive_failures",
			"next_probe_at",
			"last_observation_source",
		}),
	}
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
