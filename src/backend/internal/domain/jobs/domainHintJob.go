package cronjob

import service "github.com/alireza0/s-ui/src/backend/internal/domain/services"

type DomainHintJob struct {
	service.DomainHintService
}

func NewDomainHintJob() *DomainHintJob {
	return &DomainHintJob{}
}

func (j *DomainHintJob) Run() {
	j.DomainHintService.Refresh()
}
