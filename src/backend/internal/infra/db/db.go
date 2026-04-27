package database

import (
	"encoding/json"
	"os"
	"path"
	"strings"
	"time"

	"github.com/alireza0/b-ui/src/backend/internal/domain/config"
	"github.com/alireza0/b-ui/src/backend/internal/infra/db/model"

	"gorm.io/driver/sqlite"
	"gorm.io/gorm"
	"gorm.io/gorm/logger"
)

var db *gorm.DB

func initUser() error {
	var count int64
	err := db.Model(&model.User{}).Count(&count).Error
	if err != nil {
		return err
	}
	if count == 0 {
		user := &model.User{
			Username: "admin",
			Password: "admin",
		}
		return db.Create(user).Error
	}
	return nil
}

func OpenDB(dbPath string) error {
	dir := path.Dir(dbPath)
	err := os.MkdirAll(dir, 01740)
	if err != nil {
		return err
	}

	var gormLogger logger.Interface

	if config.IsDebug() {
		gormLogger = logger.Default
	} else {
		gormLogger = logger.Discard
	}

	c := &gorm.Config{
		Logger: gormLogger,
	}
	sep := "?"
	if strings.Contains(dbPath, "?") {
		sep = "&"
	}
	dsn := dbPath + sep + "_busy_timeout=10000&_journal_mode=WAL"
	db, err = gorm.Open(sqlite.Open(dsn), c)
	if err != nil {
		return err
	}

	sqlDB, err := db.DB()
	if err != nil {
		return err
	}
	sqlDB.SetMaxOpenConns(25)
	sqlDB.SetMaxIdleConns(5)
	sqlDB.SetConnMaxLifetime(time.Hour)

	if config.IsDebug() {
		db = db.Debug()
	}
	return nil
}

func InitDB(dbPath string) error {
	if dbPath == config.GetDBPath() {
		resolvedDBPath, err := config.PrepareDBPath()
		if err != nil {
			return err
		}
		dbPath = resolvedDBPath
	}

	err := OpenDB(dbPath)
	if err != nil {
		return err
	}

	// Default Outbounds
	if !db.Migrator().HasTable(&model.Outbound{}) {
		db.Migrator().CreateTable(&model.Outbound{})
		defaultOutbound := []model.Outbound{
			{Type: "direct", Tag: "direct", Options: json.RawMessage(`{}`)},
		}
		db.Create(&defaultOutbound)
	}

	if err := dedupeClusterPeerWorkflowState(); err != nil {
		return err
	}
	if err := dedupeClusterPeerAckState(); err != nil {
		return err
	}

	err = db.AutoMigrate(
		&model.Setting{},
		&model.Tls{},
		&model.Inbound{},
		&model.Outbound{},
		&model.Service{},
		&model.Endpoint{},
		&model.User{},
		&model.Tokens{},
		&model.Stats{},
		&model.Client{},
		&model.Changes{},
		&model.ClusterLocalNode{},
		&model.ClusterDomain{},
		&model.ClusterMember{},
		&model.ClusterPeerReachability{},
		&model.ClusterPeerEventLog{},
		&model.ClusterPeerEventState{},
		&model.ClusterPeerAckState{},
		&model.ClusterPeerWorkflowState{},
		&model.ClusterPeerSchedule{},
	)
	if err != nil {
		return err
	}
	err = initUser()
	if err != nil {
		return err
	}

	return nil
}

func dedupeClusterPeerWorkflowState() error {
	if !db.Migrator().HasTable(&model.ClusterPeerWorkflowState{}) {
		return nil
	}
	return db.Exec(`
		DELETE FROM cluster_peer_workflow_states
		WHERE EXISTS (
			SELECT 1
			FROM cluster_peer_workflow_states AS newer
			WHERE newer.workflow_id = cluster_peer_workflow_states.workflow_id
				AND newer.step_id = cluster_peer_workflow_states.step_id
				AND (
					newer.updated_at > cluster_peer_workflow_states.updated_at
					OR (
						newer.updated_at = cluster_peer_workflow_states.updated_at
						AND newer.id > cluster_peer_workflow_states.id
					)
				)
		)
	`).Error
}

func dedupeClusterPeerAckState() error {
	if !db.Migrator().HasTable(&model.ClusterPeerAckState{}) {
		return nil
	}
	return db.Exec(`
		DELETE FROM cluster_peer_ack_states
		WHERE EXISTS (
			SELECT 1
			FROM cluster_peer_ack_states AS newer
			WHERE newer.message_id = cluster_peer_ack_states.message_id
				AND newer.target_node = cluster_peer_ack_states.target_node
				AND (
					newer.updated_at > cluster_peer_ack_states.updated_at
					OR (
						newer.updated_at = cluster_peer_ack_states.updated_at
						AND newer.id > cluster_peer_ack_states.id
					)
				)
		)
	`).Error
}

func GetDB() *gorm.DB {
	return db
}

func IsNotFound(err error) bool {
	return err == gorm.ErrRecordNotFound
}
