package cronjob

import (
	"context"

	service "github.com/BeanYa/b-ui/src/backend/internal/domain/services"
	logger "github.com/BeanYa/b-ui/src/backend/internal/infra/logging"
)

type ClusterReachabilityProbeJob struct {
	prober *service.ClusterPeerProbeService
}

func NewClusterReachabilityProbeJob() *ClusterReachabilityProbeJob {
	return &ClusterReachabilityProbeJob{
		prober: service.NewRuntimeClusterPeerProbeService(),
	}
}

func (j *ClusterReachabilityProbeJob) Run() {
	if err := j.prober.ProbeIdlePeers(context.Background()); err != nil {
		logger.ClusterError(logger.ClusterCron, "reachability_probe", map[string]interface{}{
			"error": err.Error(),
		})
		return
	}
	logger.ClusterInfo(logger.ClusterCron, "reachability_probe", map[string]interface{}{
		"status": "completed",
	})
}
