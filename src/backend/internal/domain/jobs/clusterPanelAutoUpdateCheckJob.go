package cronjob

import (
	"context"

	service "github.com/BeanYa/b-ui/src/backend/internal/domain/services"
	logger "github.com/BeanYa/b-ui/src/backend/internal/infra/logging"
)

type ClusterPanelAutoUpdateCheckJob struct {
	syncService service.ClusterSyncService
}

func NewClusterPanelAutoUpdateCheckJob() *ClusterPanelAutoUpdateCheckJob {
	return &ClusterPanelAutoUpdateCheckJob{syncService: service.NewRuntimeClusterSyncService()}
}

func (j *ClusterPanelAutoUpdateCheckJob) Run() {
	err := j.syncService.CheckAutoPanelUpdates(context.Background())
	if err != nil {
		logger.ClusterError(logger.ClusterCron, "panel_auto_update_check", map[string]interface{}{
			"error": err.Error(),
		})
		return
	}
	logger.ClusterInfo(logger.ClusterCron, "panel_auto_update_check", map[string]interface{}{
		"status": "completed",
	})
}
