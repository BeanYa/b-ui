package cronjob

import "github.com/alireza0/s-ui/service"

type DomainHintJob struct {
	service.DomainHintService
}

func NewDomainHintJob() *DomainHintJob {
	return &DomainHintJob{}
}

func (j *DomainHintJob) Run() {
	j.DomainHintService.Refresh()
}
