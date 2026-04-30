package cronjob

import (
	service "github.com/BeanYa/b-ui/src/backend/internal/domain/services"
	logger "github.com/BeanYa/b-ui/src/backend/internal/infra/logging"
)

type ClusterPeerScheduleJob struct {
	service service.ClusterPeerScheduleService
}

func NewClusterPeerScheduleJob() *ClusterPeerScheduleJob {
	return &ClusterPeerScheduleJob{service: service.ClusterPeerScheduleService{}}
}

func (j *ClusterPeerScheduleJob) Run() {
	if err := j.service.RunDueSchedules(); err != nil {
		logger.ClusterError(logger.ClusterCron, "peer_schedule", map[string]interface{}{
			"error": err.Error(),
		})
		return
	}
	logger.ClusterInfo(logger.ClusterCron, "peer_schedule", map[string]interface{}{
		"status": "completed",
	})
}
