package cronjob

import service "github.com/alireza0/b-ui/src/backend/internal/domain/services"

type ClusterPeerScheduleJob struct {
	service service.ClusterPeerScheduleService
}

func NewClusterPeerScheduleJob() *ClusterPeerScheduleJob {
	return &ClusterPeerScheduleJob{service: service.ClusterPeerScheduleService{}}
}

func (j *ClusterPeerScheduleJob) Run() {
	_ = j.service.RunDueSchedules()
}
