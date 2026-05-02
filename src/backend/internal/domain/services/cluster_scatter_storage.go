package service

import (
	"time"

	"github.com/BeanYa/b-ui/src/backend/internal/infra/db/model"

	"gorm.io/gorm"
)

const scatterKeepLatest = 200
const scatterBuffer = 20

// InsertScatterTaskWithRolling inserts a task record with rolling storage enforcement.
func InsertScatterTaskWithRolling(db *gorm.DB, task *model.ClusterScatterTask) error {
	return insertWithRolling(db, task, &model.ClusterScatterTask{}, scatterKeepLatest, scatterBuffer)
}

// InsertScatterResultWithRolling inserts a result record with rolling storage enforcement.
func InsertScatterResultWithRolling(db *gorm.DB, result *model.ClusterScatterResult) error {
	return insertWithRolling(db, result, &model.ClusterScatterResult{}, scatterKeepLatest, scatterBuffer)
}

func insertWithRolling(db *gorm.DB, record any, m any, keep int, buffer int) error {
	var count int64
	if err := db.Model(m).Count(&count).Error; err != nil {
		return err
	}
	capacity := keep + buffer
	if count >= int64(capacity) {
		db.Model(m).Order("created_at ASC").Limit(buffer).Delete(m)
	}
	return db.Create(record).Error
}

// GetScatterTasksByDomain returns all tasks for a domain, newest first.
func GetScatterTasksByDomain(db *gorm.DB, domainID string) ([]model.ClusterScatterTask, error) {
	var tasks []model.ClusterScatterTask
	err := db.Where("domain_id = ?", domainID).Order("created_at DESC").Find(&tasks).Error
	return tasks, err
}

// GetScatterTaskByID returns a single task by its business task_id.
func GetScatterTaskByID(db *gorm.DB, taskID string) (*model.ClusterScatterTask, error) {
	var task model.ClusterScatterTask
	err := db.Where("task_id = ?", taskID).First(&task).Error
	if err != nil {
		return nil, err
	}
	return &task, nil
}

// UpdateScatterTaskStatus updates the status and completed_at of a task.
func UpdateScatterTaskStatus(db *gorm.DB, taskID string, status string) error {
	now := time.Now()
	return db.Model(&model.ClusterScatterTask{}).Where("task_id = ?", taskID).Updates(map[string]any{
		"status":       status,
		"completed_at": now,
	}).Error
}

// GetScatterResultByTaskID returns the aggregated result for a task.
func GetScatterResultByTaskID(db *gorm.DB, taskID string) (*model.ClusterScatterResult, error) {
	var result model.ClusterScatterResult
	err := db.Where("task_id = ?", taskID).First(&result).Error
	if err != nil {
		return nil, err
	}
	return &result, nil
}

// SaveScatterResult stores an aggregated result with rolling storage.
func SaveScatterResult(db *gorm.DB, result *model.ClusterScatterResult) error {
	return InsertScatterResultWithRolling(db, result)
}
