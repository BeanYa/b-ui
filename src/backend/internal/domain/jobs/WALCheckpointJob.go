package cronjob

import (
	database "github.com/BeanYa/b-ui/src/backend/internal/infra/db"
	logger "github.com/BeanYa/b-ui/src/backend/internal/infra/logging"
)

type WALCheckpointJob struct{}

func NewWALCheckpointJob() *WALCheckpointJob {
	return &WALCheckpointJob{}
}

func (s *WALCheckpointJob) Run() {
	db := database.GetDB()
	if err := db.Exec("PRAGMA wal_checkpoint(FULL)").Error; err != nil {
		logger.Error("Error checkpointing WAL: ", err.Error())
	}
}
