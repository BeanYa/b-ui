package cronjob

import (
	"context"

	service "github.com/BeanYa/b-ui/src/backend/internal/domain/services"
	logger "github.com/BeanYa/b-ui/src/backend/internal/infra/logging"
)

type ClusterPanelStatusReportJob struct {
	syncService service.ClusterSyncService
}

func NewClusterPanelStatusReportJob() *ClusterPanelStatusReportJob {
	return &ClusterPanelStatusReportJob{syncService: service.NewRuntimeClusterSyncService()}
}

func (j *ClusterPanelStatusReportJob) Run() {
	err := j.syncService.ReportLocalPanelStatus(context.Background())
	if err != nil {
		logger.ClusterError(logger.ClusterCron, "panel_status_report", map[string]interface{}{
			"error": err.Error(),
		})
		return
	}
	logger.ClusterInfo(logger.ClusterCron, "panel_status_report", map[string]interface{}{
		"status": "completed",
	})
}
