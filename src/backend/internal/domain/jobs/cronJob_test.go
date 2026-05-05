package cronjob

import (
	"reflect"
	"strings"
	"testing"
)

func TestScheduledCronJobsDoNotIncludeHubVersionPolling(t *testing.T) {
	jobs := scheduledCronJobs(0)
	foundPanelStatusReport := false

	for _, job := range jobs {
		if strings.Contains(reflect.TypeOf(job.job).String(), "VersionPoll") {
			t.Fatal("expected scheduled cron jobs not to poll hub versions")
		}
		if _, ok := job.job.(*ClusterPanelStatusReportJob); ok {
			foundPanelStatusReport = true
		}
	}

	if !foundPanelStatusReport {
		t.Fatal("expected node panel status report job to remain scheduled")
	}
}
