package cronjob

import (
	service "github.com/alireza0/s-ui/src/backend/internal/domain/services"
	logger "github.com/alireza0/s-ui/src/backend/internal/infra/logging"
)

type StatsJob struct {
	service.StatsService
	enableTraffic bool
}

func NewStatsJob(saveTraffic bool) *StatsJob {
	return &StatsJob{
		enableTraffic: saveTraffic,
	}
}

func (s *StatsJob) Run() {
	err := s.StatsService.SaveStats(s.enableTraffic)
	if err != nil {
		logger.Warning("Get stats failed: ", err)
		return
	}
}
