package service

import (
	"encoding/json"
	"errors"
	"strconv"
	"strings"
	"sync"
	"time"

	database "github.com/alireza0/s-ui/src/backend/internal/infra/db"
	"github.com/alireza0/s-ui/src/backend/internal/infra/db/model"
	"gorm.io/gorm"
	"gorm.io/gorm/clause"
)

const (
	PeerEventStatusReceived    = "received"
	PeerEventStatusProcessing  = "processing"
	PeerEventStatusSucceeded   = "succeeded"
	PeerEventStatusFailed      = "failed"
	PeerEventStatusIgnored     = "ignored"
	PeerEventStatusUnsupported = "unsupported"
	PeerEventStatusDead        = "dead"
)

const (
	PeerAckStatusSucceeded = "succeeded"
	PeerAckStatusFailed    = "failed"
)

type PeerEventState struct {
	MessageID   string
	PayloadHash string
	Status      string
}

type clusterPeerStore interface {
	RecordReceived(message *PeerMessage) (*PeerEventState, error)
	ClaimProcessing(messageID string) (bool, error)
	MarkEventState(messageID string, status string, errorMessage string) error
}

type dbClusterPeerStore struct{}

func newDBClusterPeerStore() clusterPeerStore {
	return &dbClusterPeerStore{}
}

func (s *dbClusterPeerStore) RecordReceived(message *PeerMessage) (*PeerEventState, error) {
	if message == nil {
		return nil, errors.New("invalid_peer_message")
	}

	db := database.GetDB()
	tx := db.Begin()
	if tx.Error != nil {
		return nil, tx.Error
	}

	existing, err := findExistingPeerEventState(tx, message)
	if err == nil {
		if existing.PayloadHash != message.PayloadHash {
			tx.Rollback()
			return nil, errors.New("payload_hash_mismatch")
		}
		if err := tx.Commit().Error; err != nil {
			return nil, err
		}
		return peerEventStateFromModel(existing), nil
	}
	if !database.IsNotFound(err) {
		tx.Rollback()
		return nil, err
	}

	now := time.Now().Unix()
	createdAt := message.CreatedAt
	if createdAt == 0 {
		createdAt = now
	}

	state := &model.ClusterPeerEventState{
		MessageID:      message.MessageID,
		IdempotencyKey: message.IdempotencyKey,
		SourceNode:     message.SourceNodeID,
		SourceSeq:      message.SourceSeq,
		DomainID:       message.DomainID,
		Action:         message.Action,
		PayloadHash:    message.PayloadHash,
		Status:         PeerEventStatusReceived,
		CreatedAt:      createdAt,
		UpdatedAt:      now,
	}
	if err := tx.Create(state).Error; err != nil {
		tx.Rollback()
		return s.resolveEventStateCreateConflict(err, message)
	}

	envelope, err := json.Marshal(message)
	if err != nil {
		tx.Rollback()
		return nil, err
	}
	log := &model.ClusterPeerEventLog{
		MessageID:   message.MessageID,
		DomainID:    message.DomainID,
		Direction:   "inbound",
		SourceNode:  message.SourceNodeID,
		Action:      message.Action,
		Envelope:    string(envelope),
		PayloadHash: message.PayloadHash,
		Signature:   message.Signature,
		CreatedAt:   createdAt,
	}
	if err := tx.Clauses(clause.OnConflict{DoNothing: true}).Create(log).Error; err != nil {
		tx.Rollback()
		return nil, err
	}

	if err := tx.Commit().Error; err != nil {
		return nil, err
	}
	return peerEventStateFromModel(state), nil
}

func (s *dbClusterPeerStore) resolveEventStateCreateConflict(createErr error, message *PeerMessage) (*PeerEventState, error) {
	state, _, err := resolvePeerEventStateCreateConflict(createErr, message, s.getEventStateByMessage)
	return state, err
}

func (s *dbClusterPeerStore) getEventStateByMessage(message *PeerMessage) (*model.ClusterPeerEventState, error) {
	return findExistingPeerEventState(database.GetDB(), message)
}

func resolvePeerEventStateCreateConflict(createErr error, message *PeerMessage, lookup func(*PeerMessage) (*model.ClusterPeerEventState, error)) (*PeerEventState, bool, error) {
	if !isUniqueConstraintError(createErr) {
		return nil, false, createErr
	}
	existing, err := lookup(message)
	if err != nil {
		return nil, true, err
	}
	if existing.PayloadHash != message.PayloadHash {
		return nil, true, errors.New("payload_hash_mismatch")
	}
	return peerEventStateFromModel(existing), true, nil
}

func findExistingPeerEventState(db interface {
	Where(query interface{}, args ...interface{}) *gorm.DB
}, message *PeerMessage) (*model.ClusterPeerEventState, error) {
	query := db.Where("message_id = ?", message.MessageID)
	if message.IdempotencyKey != "" {
		query = query.Or("domain_id = ? AND idempotency_key = ?", message.DomainID, message.IdempotencyKey)
	}
	if message.SourceNodeID != "" && message.SourceSeq > 0 {
		query = query.Or("domain_id = ? AND source_node = ? AND source_seq = ?", message.DomainID, message.SourceNodeID, message.SourceSeq)
	}
	event := &model.ClusterPeerEventState{}
	err := query.First(event).Error
	if err != nil {
		return nil, err
	}
	return event, nil
}

func isUniqueConstraintError(err error) bool {
	if err == nil {
		return false
	}
	message := strings.ToLower(err.Error())
	return strings.Contains(message, "unique constraint failed") ||
		strings.Contains(message, "duplicate key") ||
		strings.Contains(message, "duplicate entry")
}

func (s *dbClusterPeerStore) MarkEventState(messageID string, status string, errorMessage string) error {
	event := &model.ClusterPeerEventState{}
	db := database.GetDB()
	err := db.Where("message_id = ?", messageID).First(event).Error
	if err != nil {
		return err
	}
	event.Status = status
	event.Error = errorMessage
	event.UpdatedAt = time.Now().Unix()
	return db.Save(event).Error
}

func (s *dbClusterPeerStore) ClaimProcessing(messageID string) (bool, error) {
	result := database.GetDB().
		Model(&model.ClusterPeerEventState{}).
		Where("message_id = ? AND status IN ?", messageID, []string{PeerEventStatusReceived, PeerEventStatusFailed}).
		Updates(map[string]interface{}{
			"status":     PeerEventStatusProcessing,
			"error":      "",
			"updated_at": time.Now().Unix(),
		})
	if result.Error != nil {
		return false, result.Error
	}
	return result.RowsAffected == 1, nil
}

func SaveClusterPeerWorkflowStep(workflowID string, stepID string, domainID string, nodeID string, status string, resultHash string, errorMessage string) error {
	db := database.GetDB()
	now := time.Now().Unix()

	state := &model.ClusterPeerWorkflowState{
		WorkflowID: workflowID,
		StepID:     stepID,
		DomainID:   domainID,
		NodeID:     nodeID,
		Status:     status,
		ResultHash: resultHash,
		Error:      errorMessage,
		CreatedAt:  now,
		UpdatedAt:  now,
	}

	return db.Clauses(clause.OnConflict{
		Columns: []clause.Column{
			{Name: "workflow_id"},
			{Name: "step_id"},
		},
		DoUpdates: clause.AssignmentColumns([]string{
			"domain_id",
			"node_id",
			"status",
			"result_hash",
			"error",
			"updated_at",
		}),
	}).Create(state).Error
}

func SaveClusterPeerAckAttempt(messageID string, targetNode string, status string, errorMessage string) error {
	if messageID == "" || targetNode == "" {
		return nil
	}
	db := database.GetDB()
	if db == nil || db.Config == nil {
		return nil
	}
	now := time.Now().Unix()
	state := &model.ClusterPeerAckState{
		MessageID:  messageID,
		TargetNode: targetNode,
		Status:     status,
		Attempts:   1,
		Error:      errorMessage,
		UpdatedAt:  now,
	}
	return db.Clauses(clause.OnConflict{
		Columns: []clause.Column{
			{Name: "message_id"},
			{Name: "target_node"},
		},
		DoUpdates: clause.Assignments(map[string]interface{}{
			"status":     status,
			"attempts":   gorm.Expr("cluster_peer_ack_states.attempts + 1"),
			"error":      errorMessage,
			"updated_at": now,
		}),
	}).Create(state).Error
}

func peerEventStateFromModel(event *model.ClusterPeerEventState) *PeerEventState {
	return &PeerEventState{
		MessageID:   event.MessageID,
		PayloadHash: event.PayloadHash,
		Status:      event.Status,
	}
}

type memoryPeerStore struct {
	mu              sync.Mutex
	states          map[string]*PeerEventState
	idempotencyKeys map[string]string
	sourceSequences map[string]string
}

func newMemoryPeerStore() *memoryPeerStore {
	return &memoryPeerStore{
		states:          make(map[string]*PeerEventState),
		idempotencyKeys: make(map[string]string),
		sourceSequences: make(map[string]string),
	}
}

func (s *memoryPeerStore) RecordReceived(message *PeerMessage) (*PeerEventState, error) {
	if message == nil {
		return nil, errors.New("invalid_peer_message")
	}

	s.mu.Lock()
	defer s.mu.Unlock()

	if existing, ok := s.states[message.MessageID]; ok {
		return cloneMatchingPeerEventState(existing, message)
	}
	if message.IdempotencyKey != "" {
		if messageID, ok := s.idempotencyKeys[memoryPeerIdempotencyKey(message.DomainID, message.IdempotencyKey)]; ok {
			return cloneMatchingPeerEventState(s.states[messageID], message)
		}
	}
	if message.SourceNodeID != "" && message.SourceSeq > 0 {
		if messageID, ok := s.sourceSequences[memoryPeerSourceSeqKey(message.DomainID, message.SourceNodeID, message.SourceSeq)]; ok {
			return cloneMatchingPeerEventState(s.states[messageID], message)
		}
	}

	state := &PeerEventState{
		MessageID:   message.MessageID,
		PayloadHash: message.PayloadHash,
		Status:      PeerEventStatusReceived,
	}
	s.states[message.MessageID] = state
	if message.IdempotencyKey != "" {
		s.idempotencyKeys[memoryPeerIdempotencyKey(message.DomainID, message.IdempotencyKey)] = message.MessageID
	}
	if message.SourceNodeID != "" && message.SourceSeq > 0 {
		s.sourceSequences[memoryPeerSourceSeqKey(message.DomainID, message.SourceNodeID, message.SourceSeq)] = message.MessageID
	}
	return clonePeerEventState(state), nil
}

func cloneMatchingPeerEventState(existing *PeerEventState, message *PeerMessage) (*PeerEventState, error) {
	if existing == nil {
		return nil, errors.New("peer_event_not_found")
	}
	if existing.PayloadHash != message.PayloadHash {
		return nil, errors.New("payload_hash_mismatch")
	}
	return clonePeerEventState(existing), nil
}

func (s *memoryPeerStore) ClaimProcessing(messageID string) (bool, error) {
	s.mu.Lock()
	defer s.mu.Unlock()

	state, ok := s.states[messageID]
	if !ok {
		return false, errors.New("peer_event_not_found")
	}
	if state.Status != PeerEventStatusReceived && state.Status != PeerEventStatusFailed {
		return false, nil
	}
	state.Status = PeerEventStatusProcessing
	return true, nil
}

func (s *memoryPeerStore) MarkEventState(messageID string, status string, errorMessage string) error {
	s.mu.Lock()
	defer s.mu.Unlock()

	state, ok := s.states[messageID]
	if !ok {
		return errors.New("peer_event_not_found")
	}
	state.Status = status
	return nil
}

func clonePeerEventState(state *PeerEventState) *PeerEventState {
	return &PeerEventState{
		MessageID:   state.MessageID,
		PayloadHash: state.PayloadHash,
		Status:      state.Status,
	}
}

func memoryPeerIdempotencyKey(domainID string, idempotencyKey string) string {
	return domainID + "\x00" + idempotencyKey
}

func memoryPeerSourceSeqKey(domainID string, sourceNodeID string, sourceSeq int64) string {
	return domainID + "\x00" + sourceNodeID + "\x00" + strconv.FormatInt(sourceSeq, 10)
}
