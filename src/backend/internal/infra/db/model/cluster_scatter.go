package model

import "time"

// ClusterScatterTask represents a scatter-gather task record.
type ClusterScatterTask struct {
	ID          uint       `gorm:"primaryKey"`
	TaskID      string     `gorm:"uniqueIndex;not null"`
	DomainID    string     `gorm:"index;not null"`
	TaskType    string     `gorm:"not null"`
	Scope       string     `gorm:"not null"`
	Status      string     `gorm:"not null"`
	ParamsJSON  string     `gorm:"type:text"`
	CreatedAt   time.Time
	CompletedAt *time.Time
}

func (ClusterScatterTask) TableName() string { return "cluster_scatter_tasks" }

// ClusterScatterResult stores the aggregated result of a completed task.
type ClusterScatterResult struct {
	ID          uint      `gorm:"primaryKey"`
	TaskID      string    `gorm:"uniqueIndex;not null"`
	ReportJSON  string    `gorm:"type:text"`
	GeneratedAt time.Time
}

func (ClusterScatterResult) TableName() string { return "cluster_scatter_results" }
