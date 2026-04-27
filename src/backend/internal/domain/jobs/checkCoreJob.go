package cronjob

import (
	service "github.com/BeanYa/b-ui/src/backend/internal/domain/services"
)

type CheckCoreJob struct {
	service.ConfigService
}

func NewCheckCoreJob() *CheckCoreJob {
	return &CheckCoreJob{}
}

func (s *CheckCoreJob) Run() {
	s.ConfigService.StartCore()
}
