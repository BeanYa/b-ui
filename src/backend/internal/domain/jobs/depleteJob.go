package cronjob

import (
	service "github.com/BeanYa/b-ui/src/backend/internal/domain/services"
	database "github.com/BeanYa/b-ui/src/backend/internal/infra/db"
	logger "github.com/BeanYa/b-ui/src/backend/internal/infra/logging"
)

type DepleteJob struct {
	service.ClientService
	service.InboundService
}

func NewDepleteJob() *DepleteJob {
	return new(DepleteJob)
}

func (s *DepleteJob) Run() {
	inboundIds, err := s.ClientService.DepleteClients()
	if err != nil {
		logger.Warning("Disable depleted users failed: ", err)
		return
	}
	if len(inboundIds) > 0 {
		err := s.InboundService.RestartInbounds(database.GetDB(), inboundIds)
		if err != nil {
			logger.Error("unable to restart inbounds: ", err)
		}
	}
}
