package service

import (
	"encoding/json"
	"errors"
	"strings"
	"time"

	database "github.com/BeanYa/b-ui/src/backend/internal/infra/db"
	"github.com/BeanYa/b-ui/src/backend/internal/infra/db/model"
	"gorm.io/gorm"
	"gorm.io/gorm/clause"
)

const (
	ClusterDomainResourceInbound = "domain_inbound"
	ClusterDomainResourceUser    = "domain_user"
)

const (
	ClusterDomainOperationCreate = "create"
	ClusterDomainOperationUpdate = "update"
	ClusterDomainOperationDelete = "delete"
)

const (
	ClusterDomainOperationQueued      = "queued"
	ClusterDomainOperationDispatching = "dispatching"
	ClusterDomainOperationPartial     = "partial"
	ClusterDomainOperationApplied     = "applied"
	ClusterDomainOperationFailed      = "failed"
	ClusterDomainOperationTimeout     = "timeout"
	ClusterDomainOperationSkipped     = "skipped"
	ClusterDomainOperationReported    = "reported"
)

type ClusterDomainOperationSummary struct {
	Queued  int `json:"queued"`
	Applied int `json:"applied"`
	Failed  int `json:"failed"`
	Timeout int `json:"timeout"`
	Skipped int `json:"skipped"`
	Total   int `json:"total"`
}

type ClusterDomainOperationInstanceView struct {
	MemberID        string `json:"memberId"`
	NodeID          string `json:"nodeId"`
	DisplayName     string `json:"displayName"`
	TargetTag       string `json:"targetTag"`
	Status          string `json:"status"`
	AttemptCount    int    `json:"attemptCount"`
	LocalResourceID uint   `json:"localResourceId"`
	Error           string `json:"error"`
	UpdatedAt       int64  `json:"updatedAt"`
}

type ClusterDomainOperationView struct {
	OperationID  string                               `json:"operationId"`
	DomainID     uint                                 `json:"domainId"`
	Domain       string                               `json:"domain"`
	ResourceKind string                               `json:"resourceKind"`
	ResourceID   string                               `json:"resourceId"`
	Action       string                               `json:"action"`
	Revision     int64                                `json:"revision"`
	Status       string                               `json:"status"`
	Summary      ClusterDomainOperationSummary        `json:"summary"`
	Instances    []ClusterDomainOperationInstanceView `json:"instances"`
	UpdatedAt    int64                                `json:"updatedAt"`
}

type ClusterDomainOperationStore struct {
	DB *gorm.DB
}

func (s ClusterDomainOperationStore) db() *gorm.DB {
	if s.DB != nil {
		return s.DB
	}
	return database.GetDB()
}

func (s ClusterDomainOperationStore) SaveOperation(op *model.ClusterDomainOperation) error {
	if op == nil {
		return errors.New("cluster domain operation is required")
	}
	op.OperationID = strings.TrimSpace(op.OperationID)
	if op.OperationID == "" {
		return errors.New("operation_id is required")
	}
	db := s.db()
	if db == nil {
		return errors.New("cluster domain operation db is required")
	}
	now := time.Now().Unix()
	if op.CreatedAt == 0 {
		op.CreatedAt = now
	}
	op.UpdatedAt = now

	insert := *op
	insert.Id = 0
	if err := db.Clauses(clause.OnConflict{
		Columns: []clause.Column{{Name: "operation_id"}},
		DoUpdates: clause.AssignmentColumns([]string{
			"domain_id",
			"domain",
			"resource_kind",
			"resource_id",
			"action",
			"revision",
			"coordinator_node_id",
			"status",
			"desired_payload",
			"summary_json",
			"updated_at",
		}),
	}).Create(&insert).Error; err != nil {
		return err
	}
	persisted, err := s.GetOperation(op.OperationID)
	if err != nil {
		return err
	}
	*op = *persisted
	return nil
}

func (s ClusterDomainOperationStore) SaveInstance(instance *model.ClusterDomainOperationInstance) error {
	if instance == nil {
		return errors.New("cluster domain operation instance is required")
	}
	instance.OperationID = strings.TrimSpace(instance.OperationID)
	instance.NodeID = strings.TrimSpace(instance.NodeID)
	if instance.OperationID == "" {
		return errors.New("operation_id is required")
	}
	if instance.NodeID == "" {
		return errors.New("node_id is required")
	}
	db := s.db()
	if db == nil {
		return errors.New("cluster domain operation db is required")
	}
	now := time.Now().Unix()
	if instance.CreatedAt == 0 {
		instance.CreatedAt = now
	}
	instance.UpdatedAt = now

	insert := *instance
	insert.Id = 0
	if err := db.Clauses(clause.OnConflict{
		Columns: []clause.Column{
			{Name: "operation_id"},
			{Name: "node_id"},
		},
		DoUpdates: clause.AssignmentColumns([]string{
			"member_id",
			"display_name",
			"target_tag",
			"status",
			"attempt_count",
			"local_resource_id",
			"response_json",
			"error",
			"last_attempt_at",
			"updated_at",
		}),
	}).Create(&insert).Error; err != nil {
		return err
	}
	persisted := &model.ClusterDomainOperationInstance{}
	if err := db.Where("operation_id = ? AND node_id = ?", instance.OperationID, instance.NodeID).First(persisted).Error; err != nil {
		return err
	}
	*instance = *persisted
	return nil
}

func (s ClusterDomainOperationStore) GetOperation(operationID string) (*model.ClusterDomainOperation, error) {
	operationID = strings.TrimSpace(operationID)
	if operationID == "" {
		return nil, errors.New("operation_id is required")
	}
	db := s.db()
	if db == nil {
		return nil, errors.New("cluster domain operation db is required")
	}
	op := &model.ClusterDomainOperation{}
	if err := db.Where("operation_id = ?", operationID).First(op).Error; err != nil {
		return nil, err
	}
	return op, nil
}

func (s ClusterDomainOperationStore) ListInstances(operationID string) ([]model.ClusterDomainOperationInstance, error) {
	operationID = strings.TrimSpace(operationID)
	if operationID == "" {
		return nil, errors.New("operation_id is required")
	}
	db := s.db()
	if db == nil {
		return nil, errors.New("cluster domain operation db is required")
	}
	var instances []model.ClusterDomainOperationInstance
	if err := db.Where("operation_id = ?", operationID).Order("node_id asc").Find(&instances).Error; err != nil {
		return nil, err
	}
	return instances, nil
}

func (s ClusterDomainOperationStore) RecomputeStatus(operationID string) (string, ClusterDomainOperationSummary, error) {
	op, err := s.GetOperation(operationID)
	if err != nil {
		return "", ClusterDomainOperationSummary{}, err
	}
	instances, err := s.ListInstances(operationID)
	if err != nil {
		return "", ClusterDomainOperationSummary{}, err
	}

	summary := summarizeClusterDomainOperationInstances(instances)
	status := statusForClusterDomainOperationSummary(summary, op.Status)

	summaryJSON, err := json.Marshal(summary)
	if err != nil {
		return "", summary, err
	}
	op.Status = status
	op.SummaryJSON = summaryJSON
	op.UpdatedAt = time.Now().Unix()
	if err := s.db().Save(op).Error; err != nil {
		return "", summary, err
	}
	return status, summary, nil
}

func (s ClusterDomainOperationStore) GetOperationView(operationID string) (*ClusterDomainOperationView, error) {
	op, err := s.GetOperation(operationID)
	if err != nil {
		return nil, err
	}
	instances, err := s.ListInstances(operationID)
	if err != nil {
		return nil, err
	}
	summary := ClusterDomainOperationSummary{}
	if len(op.SummaryJSON) > 0 {
		_ = json.Unmarshal(op.SummaryJSON, &summary)
	}
	if summary.Total == 0 && len(instances) > 0 {
		summary = summarizeClusterDomainOperationInstances(instances)
	}

	view := &ClusterDomainOperationView{
		OperationID:  op.OperationID,
		DomainID:     op.DomainID,
		Domain:       op.Domain,
		ResourceKind: op.ResourceKind,
		ResourceID:   op.ResourceID,
		Action:       op.Action,
		Revision:     op.Revision,
		Status:       op.Status,
		Summary:      summary,
		Instances:    make([]ClusterDomainOperationInstanceView, 0, len(instances)),
		UpdatedAt:    op.UpdatedAt,
	}
	for _, instance := range instances {
		view.Instances = append(view.Instances, ClusterDomainOperationInstanceView{
			MemberID:        instance.MemberID,
			NodeID:          instance.NodeID,
			DisplayName:     instance.DisplayName,
			TargetTag:       instance.TargetTag,
			Status:          instance.Status,
			AttemptCount:    instance.AttemptCount,
			LocalResourceID: instance.LocalResourceID,
			Error:           instance.Error,
			UpdatedAt:       instance.UpdatedAt,
		})
	}
	return view, nil
}

func summarizeClusterDomainOperationInstances(instances []model.ClusterDomainOperationInstance) ClusterDomainOperationSummary {
	summary := ClusterDomainOperationSummary{Total: len(instances)}
	for _, instance := range instances {
		switch instance.Status {
		case ClusterDomainOperationApplied:
			summary.Applied++
		case ClusterDomainOperationFailed:
			summary.Failed++
		case ClusterDomainOperationTimeout:
			summary.Timeout++
		case ClusterDomainOperationSkipped:
			summary.Skipped++
		default:
			summary.Queued++
		}
	}
	return summary
}

func statusForClusterDomainOperationSummary(summary ClusterDomainOperationSummary, currentStatus string) string {
	if summary.Total == 0 {
		if isPreservableZeroInstanceOperationStatus(currentStatus) {
			return currentStatus
		}
		return ClusterDomainOperationQueued
	}
	if summary.Skipped == summary.Total {
		return ClusterDomainOperationSkipped
	}
	if summary.Timeout == summary.Total {
		return ClusterDomainOperationTimeout
	}
	if summary.Failed == summary.Total {
		return ClusterDomainOperationFailed
	}
	if summary.Failed > 0 || summary.Timeout > 0 || summary.Queued > 0 {
		return ClusterDomainOperationPartial
	}
	return ClusterDomainOperationApplied
}

func isPreservableZeroInstanceOperationStatus(status string) bool {
	switch status {
	case ClusterDomainOperationQueued,
		ClusterDomainOperationDispatching,
		ClusterDomainOperationPartial,
		ClusterDomainOperationReported:
		return true
	default:
		return false
	}
}
