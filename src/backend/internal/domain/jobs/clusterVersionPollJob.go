package cronjob

import (
	"context"

	service "github.com/alireza0/s-ui/src/backend/internal/domain/services"
)

type ClusterVersionPollJob struct {
	syncService service.ClusterSyncService
}

func NewClusterVersionPollJob() *ClusterVersionPollJob {
	return &ClusterVersionPollJob{syncService: service.NewRuntimeClusterSyncService()}
}

func (j *ClusterVersionPollJob) Run() {
	_ = j.syncService.PollAndNotifyVersion(context.Background())
}
