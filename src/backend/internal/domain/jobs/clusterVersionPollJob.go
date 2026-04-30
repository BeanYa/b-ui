package cronjob

import (
	"context"

	service "github.com/BeanYa/b-ui/src/backend/internal/domain/services"
	logger "github.com/BeanYa/b-ui/src/backend/internal/infra/logging"
)

type ClusterVersionPollJob struct {
	syncService service.ClusterSyncService
}

func NewClusterVersionPollJob() *ClusterVersionPollJob {
	return &ClusterVersionPollJob{syncService: service.NewRuntimeClusterSyncService()}
}

func (j *ClusterVersionPollJob) Run() {
	err := j.syncService.PollAndNotifyVersion(context.Background())
	if err != nil {
		logger.ClusterError(logger.ClusterCron, "version_poll", map[string]interface{}{
			"error": err.Error(),
		})
		return
	}
	logger.ClusterInfo(logger.ClusterCron, "version_poll", map[string]interface{}{
		"status": "completed",
	})
}
