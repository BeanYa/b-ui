package cronjob

import service "github.com/alireza0/s-ui/src/backend/internal/domain/services"

type ClusterPeerScheduleJob struct {
	service service.ClusterPeerScheduleService
}

func NewClusterPeerScheduleJob() *ClusterPeerScheduleJob {
	return &ClusterPeerScheduleJob{service: service.ClusterPeerScheduleService{}}
}

func (j *ClusterPeerScheduleJob) Run() {
	_ = j.service.RunDueSchedules()
}
