package cronjob

import (
	"context"

	service "github.com/alireza0/s-ui/src/backend/internal/domain/services"
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
	_ = j.prober.ProbeIdlePeers(context.Background())
}
