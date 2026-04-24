package service

import (
	"encoding/json"
	"errors"
	"strings"
	"sync"
	"time"

	database "github.com/alireza0/s-ui/src/backend/internal/infra/db"
	"github.com/alireza0/s-ui/src/backend/internal/infra/db/model"
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

type PeerEventState struct {
	MessageID   string
	PayloadHash string
	Status      string
}

type clusterPeerStore interface {
	RecordReceived(message *PeerMessage) (*PeerEventState, error)
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

	existing := &model.ClusterPeerEventState{}
	err := tx.Where("message_id = ?", message.MessageID).First(existing).Error
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
	state, _, err := resolvePeerEventStateCreateConflict(createErr, message, s.getEventStateByMessageID)
	return state, err
}

func (s *dbClusterPeerStore) getEventStateByMessageID(messageID string) (*model.ClusterPeerEventState, error) {
	event := &model.ClusterPeerEventState{}
	err := database.GetDB().Where("message_id = ?", messageID).First(event).Error
	if err != nil {
		return nil, err
	}
	return event, nil
}

func resolvePeerEventStateCreateConflict(createErr error, message *PeerMessage, lookup func(messageID string) (*model.ClusterPeerEventState, error)) (*PeerEventState, bool, error) {
	if !isUniqueConstraintError(createErr) {
		return nil, false, createErr
	}
	existing, err := lookup(message.MessageID)
	if err != nil {
		return nil, true, err
	}
	if existing.PayloadHash != message.PayloadHash {
		return nil, true, errors.New("payload_hash_mismatch")
	}
	return peerEventStateFromModel(existing), true, nil
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

func peerEventStateFromModel(event *model.ClusterPeerEventState) *PeerEventState {
	return &PeerEventState{
		MessageID:   event.MessageID,
		PayloadHash: event.PayloadHash,
		Status:      event.Status,
	}
}

type memoryPeerStore struct {
	mu     sync.Mutex
	states map[string]*PeerEventState
}

func newMemoryPeerStore() *memoryPeerStore {
	return &memoryPeerStore{
		states: make(map[string]*PeerEventState),
	}
}

func (s *memoryPeerStore) RecordReceived(message *PeerMessage) (*PeerEventState, error) {
	if message == nil {
		return nil, errors.New("invalid_peer_message")
	}

	s.mu.Lock()
	defer s.mu.Unlock()

	if existing, ok := s.states[message.MessageID]; ok {
		if existing.PayloadHash != message.PayloadHash {
			return nil, errors.New("payload_hash_mismatch")
		}
		return clonePeerEventState(existing), nil
	}

	state := &PeerEventState{
		MessageID:   message.MessageID,
		PayloadHash: message.PayloadHash,
		Status:      PeerEventStatusReceived,
	}
	s.states[message.MessageID] = state
	return clonePeerEventState(state), nil
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
