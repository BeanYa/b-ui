package cronjob

import (
	"context"

	service "github.com/alireza0/s-ui/src/backend/internal/domain/services"
	logger "github.com/alireza0/s-ui/src/backend/internal/infra/logging"
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
		logger.Warning("Cluster reachability probe failed: ", err)
	}
}
